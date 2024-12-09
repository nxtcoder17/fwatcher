package watcher

import (
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
	WatchEvents(func(event Event, fp string) error)
}

type eventInfo struct {
	Time     time.Time
	FileInfo os.FileInfo
	Counter  int
}

type fsnWatcher struct {
	watcher  *fsnotify.Watcher
	eventMap map[string]eventInfo

	directoryCount int

	Logger            *slog.Logger
	OnlyWatchSuffixes []string
	IgnoreSuffixes    []string
	ExcludeDirs       map[string]struct{}
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
	if event.Op == fsnotify.Chmod {
		return true, "event is of type CHMOD"
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

	for _, suffix := range f.IgnoreSuffixes {
		if strings.HasSuffix(event.Name, suffix) {
			f.Logger.Debug("file is ignored", "file", event.Name)
			return true, fmt.Sprintf("because, file has suffix (%s), which is in ignore suffixes array(%+v)", suffix, f.IgnoreSuffixes)
		}
	}

	if len(f.OnlyWatchSuffixes) == 0 {
		return false, "event not in ignore list, and only-watch list is also empty"
	}

	matched := false
	for _, suffix := range f.OnlyWatchSuffixes {
		f.Logger.Debug("[only-watch-suffix] suffix: (%s), event.name: %s", suffix, event.Name)
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

func (f *fsnWatcher) WatchEvents(watcherFunc func(event Event, fp string) error) {
	// f.eventMap = map[string]eventInfo{}
	lastProcessingTime := time.Now()
	for {
		select {
		case event, ok := <-f.watcher.Events:
			{
				if !ok {
					return
				}

				t := time.Now()
				f.Logger.Debug(fmt.Sprintf("event %+v received", event))

				if ignore, reason := f.ignoreEvent(event); ignore {
					f.Logger.Debug("IGNORING", "event.name", event.Name, "reason", reason)
					continue
				}

				f.Logger.Debug("PROCESSING", "event.name", event.Name, "event.op", event.Op.String())

				// _, err := os.Stat(event.Name)
				// if err != nil {
				// 	return
				// }

				// eInfo, ok := f.eventMap[event.Name]
				// if !ok {
				// 	eInfo = eventInfo{Time: time.Now(), FileInfo: nil, Counter: 0}
				// }
				// eInfo.Counter += 1
				// f.eventMap[event.Name] = eInfo

				if time.Since(lastProcessingTime) < 100*time.Millisecond {
					// f.Logger.Debug("too many events under 100ms, ignoring...", "counter", eInfo.Counter)
					continue
				}

				if err := watcherFunc(Event(event), event.Name); err != nil {
					f.Logger.Error("while processing event, got", "err", err)
					return
				}
				// eInfo.Time = time.Now()
				// eInfo.Counter = 0
				// f.eventMap[event.Name] = eInfo

				f.Logger.Debug("watch loop completed", "took", fmt.Sprintf("%dms", time.Since(t).Milliseconds()))
			}
		case err, ok := <-f.watcher.Errors:
			if !ok {
				return
			}
			f.Logger.Error("watcher error", "err", err)
		}
	}
}

func (f *fsnWatcher) RecursiveAdd(dirs ...string) error {
	for _, dir := range dirs {
		fi, err := os.Lstat(dir)
		if err != nil {
			return err
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
	Logger               *slog.Logger
	OnlyWatchSuffixes    []string
	IgnoreSuffixes       []string
	ExcludeDirs          []string
	UseDefaultIgnoreList bool
}

func NewWatcher(args WatcherArgs) (Watcher, error) {
	if args.Logger == nil {
		args.Logger = slog.Default()
	}

	if args.UseDefaultIgnoreList {
		args.ExcludeDirs = append(args.ExcludeDirs, globalExcludeDirs...)
	}

	excludeDirs := map[string]struct{}{}
	for _, dir := range args.ExcludeDirs {
		excludeDirs[dir] = struct{}{}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		args.Logger.Error("failed to create watcher, got", "err", err)
		return nil, err
	}
	return &fsnWatcher{
		watcher:           watcher,
		Logger:            args.Logger,
		ExcludeDirs:       excludeDirs,
		IgnoreSuffixes:    args.IgnoreSuffixes,
		OnlyWatchSuffixes: args.OnlyWatchSuffixes,
	}, nil
}
