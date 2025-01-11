package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Watcher interface {
	Close() error
	RecursiveAdd(dir ...string) error
	Watch(ctx context.Context)
	GetEvents() chan Event
}

type fsnWatcher struct {
	watcher *fsnotify.Watcher

	directoryCount int

	Logger         *slog.Logger
	OnlySuffixes   []string
	IgnoreSuffixes []string
	ExcludeDirs    map[string]struct{}
	watchingDirs   map[string]struct{}

	cooldownDuration time.Duration

	eventsCh chan Event
}

// GetEvents implements Watcher.
func (f *fsnWatcher) GetEvents() chan Event {
	return f.eventsCh
}

type Event fsnotify.Event

var (
	Create = fsnotify.Create
	Delete = fsnotify.Remove
	Update = fsnotify.Write
	Rename = fsnotify.Rename
	Chmod  = fsnotify.Chmod
)

func (f fsnWatcher) ignoreEvent(event fsnotify.Event) (ignore bool, reason string) {
	// INFO: any file change emits a chain of events, but
	// we can always expect a Write event out of that event chain
	if event.Op != fsnotify.Write {
		return true, fmt.Sprintf("event (%s) is not of type WRITE", event.Op)
	}

	// Vim/Neovim creates this temporary file to see whether it can write
	// into a target directory. It screws up our watching algorithm,
	// so ignore it.
	// [source](https://brandur.org/live-reload)
	if filepath.Base(event.Name) == "4913" {
		return true, "event is from a temporary file created by vim/neovim"
	}

	// Special case for Vim:
	// vim creates files with ~ suffixes, which we don't want to watch.
	if strings.HasSuffix(event.Name, "~") {
		return true, "event is from a special file from vim/neovim which ends in ~"
	}

	for k := range f.ExcludeDirs {
		if strings.Contains(event.Name, k) {
			return true, "event is generating from an excluded path"
		}
	}

	for _, suffix := range f.IgnoreSuffixes {
		if strings.HasSuffix(event.Name, suffix) {
			f.Logger.Debug("file is ignored", "file", event.Name)
			return true, fmt.Sprintf("because, file has suffix (%s), which is in ignore suffixes array(%+v)", suffix, f.IgnoreSuffixes)
		}
	}

	if len(f.OnlySuffixes) == 0 {
		return false, "event not in ignore list, and only-watch list is also empty"
	}

	matched := false
	for _, suffix := range f.OnlySuffixes {
		f.Logger.Debug(fmt.Sprintf("[only-suffix] suffix: (%s), event.name: %s", suffix, event.Name))
		if strings.HasSuffix(event.Name, suffix) {
			matched = true
			break
		}
	}
	if matched {
		return false, "event suffix is present in only-watch-suffixes"
	}

	return true, "event ignored as suffix is not present in only-watch-suffixes"
}

func (f *fsnWatcher) Watch(ctx context.Context) {
	lastProcessingTime := time.Now()

	for {
		select {
		case event, ok := <-f.watcher.Events:
			{
				if !ok {
					return
				}

				if event.Op == fsnotify.Create {
					fi, _ := os.Stat(event.Name)
					if fi != nil && fi.IsDir() {
						skip := false

						for k := range f.ExcludeDirs {
							if strings.Contains(event.Name, k) {
								skip = true
								break
							}
						}

						if !skip {
							f.RecursiveAdd(event.Name)
						}
					}
				}

				t := time.Now()
				f.Logger.Debug(fmt.Sprintf("event %+v received", event))

				if ignore, reason := f.ignoreEvent(event); ignore {
					f.Logger.Debug("IGNORING", "event.name", event.Name, "reason", reason)
					continue
				}

				f.Logger.Debug("PROCESSING", "event.name", event.Name, "event.op", event.Op.String())

				if time.Since(lastProcessingTime) < f.cooldownDuration {
					f.Logger.Debug(fmt.Sprintf("too many events under %s, ignoring...", f.cooldownDuration.String()), "event.name", event.Name)
					continue
				}

				f.eventsCh <- Event(event)

				f.Logger.Debug("watch loop completed", "took", fmt.Sprintf("%dms", time.Since(t).Milliseconds()))
			}

		case <-ctx.Done():
			f.Logger.Debug("watcher is closing", "reason", "context closed")
			close(f.eventsCh)
			f.watcher.Close()
			return
		}
	}
}

func (f *fsnWatcher) RecursiveAdd(dirs ...string) error {
	for _, dir := range dirs {
		if _, ok := f.watchingDirs[dir]; ok {
			continue
		}

		f.watchingDirs[dir] = struct{}{}

		fi, err := os.Lstat(dir)
		if err != nil {
			continue
			// INFO: instead of returning and error, seems like ignore is a better choice
			// return err
		}

		if !fi.IsDir() {
			continue
		}

		if _, ok := f.ExcludeDirs[filepath.Base(dir)]; ok {
			f.Logger.Debug("EXCLUDED from watchlist", "dir", dir)
			continue
		}

		f.addToWatchList(dir)

		ls, err := os.ReadDir(dir)
		if err != nil {
			return err
		}

		de := make([]string, 0, len(ls))
		for _, l := range ls { // TODO: use filepath.WalkDir
			if !l.IsDir() {
				continue
			}
			de = append(de, filepath.Join(dir, l.Name()))
		}

		f.RecursiveAdd(de...)
	}

	return nil
}

func (f *fsnWatcher) addToWatchList(dir string) error {
	if err := f.watcher.Add(dir); err != nil {
		f.Logger.Error("failed to add directory", "dir", dir, "err", err)
		return err
	}
	f.directoryCount++
	f.Logger.Debug("ADDED to watchlist", "dir", dir, "count", f.directoryCount)
	return nil
}

func (f *fsnWatcher) Close() error {
	return f.watcher.Close()
}

type WatcherArgs struct {
	Logger *slog.Logger

	WatchDirs        []string
	WatchExtensions  []string
	IgnoreExtensions []string
	IgnoreDirs       []string

	IgnoreList []string

	CooldownDuration *time.Duration
	Interactive      bool
}

func NewWatcher(ctx context.Context, args WatcherArgs) (Watcher, error) {
	if args.Logger == nil {
		args.Logger = slog.Default()
	}

	args.IgnoreDirs = append(args.IgnoreDirs, args.IgnoreList...)

	cooldown := 500 * time.Millisecond

	if args.CooldownDuration != nil {
		cooldown = *args.CooldownDuration
	}

	excludeDirs := map[string]struct{}{}
	for _, dir := range args.IgnoreDirs {
		args.Logger.Debug("EXCLUDED from watching", "dir", dir)
		excludeDirs[dir] = struct{}{}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		args.Logger.Error("failed to create watcher, got", "err", err)
		return nil, err
	}

	if args.WatchDirs == nil {
		dir, _ := os.Getwd()
		args.WatchDirs = append(args.WatchDirs, dir)
	}

	fsw := &fsnWatcher{
		watcher:          watcher,
		Logger:           args.Logger,
		ExcludeDirs:      excludeDirs,
		IgnoreSuffixes:   args.IgnoreExtensions,
		OnlySuffixes:     args.WatchExtensions,
		cooldownDuration: cooldown,
		watchingDirs:     make(map[string]struct{}),

		eventsCh: make(chan Event),
	}

	if err := fsw.RecursiveAdd(args.WatchDirs...); err != nil {
		return nil, err
	}

	return fsw, nil
}
