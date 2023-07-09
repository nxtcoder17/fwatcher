package fs_watcher

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/nxtcoder17/fwatcher/pkg/logging"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Watcher interface {
	Close() error
	Add(dir ...string) error
	RecursiveAdd(dir ...string) error
	WatchEvents(func(event Event, fp string) error)
}

type eventInfo struct {
	Time     time.Time
	FileInfo os.FileInfo
	Counter  int
}

type fsnWatcher struct {
	watcher          *fsnotify.Watcher
	eventMap         map[string]eventInfo
	logger           logging.Logger
	watchExtensions  []string
	ignoreExtensions []string
	excludeDirs      []string
}

type Event fsnotify.Event

var (
	Create = fsnotify.Create
	Delete = fsnotify.Remove
	Update = fsnotify.Write
	Rename = fsnotify.Rename
	Chmod  = fsnotify.Chmod
)

func (f fsnWatcher) WatchEvents(watcherFunc func(event Event, fp string) error) {
	f.eventMap = map[string]eventInfo{}
	for {
		select {
		case event, ok := <-f.watcher.Events:
			{
				if !ok {
					return
				}

				t := time.Now()
				f.logger.Debug(fmt.Sprintf("event %+v received", event))

				shouldIgnore := false

				for _, v := range f.ignoreExtensions {
					if strings.HasSuffix(event.Name, v) {
						f.logger.Debug(fmt.Sprintf("event occured on file %q, ignoring due to ignored extension: %q", event.Name, v))
						shouldIgnore = true
						break
					}
				}

				for _, v := range f.excludeDirs {
					absV := v
					if !filepath.IsAbs(v) {
						cwd, _ := os.Getwd()
						absV = filepath.Join(cwd, v)
					}

					if strings.HasPrefix(event.Name, absV) {
						f.logger.Debug(fmt.Sprintf("event occured on file %q, ignoring due to excluded directory: %q", event.Name, v))
						shouldIgnore = true
					}
				}

				if shouldIgnore {
					continue
				}

				shouldWatch := true
				if len(f.watchExtensions) > 0 {
					shouldWatch = false
				}

				for i := range f.watchExtensions {
					if strings.HasSuffix(event.Name, f.watchExtensions[i]) {
						shouldWatch = true
						break
					}
				}

				if !shouldWatch {
					continue
				}

				eInfo, ok := f.eventMap[event.Name]
				if !ok {
					eInfo = eventInfo{Time: time.Time{}, FileInfo: nil, Counter: 0}
				}
				eInfo.Counter += 1
				f.eventMap[event.Name] = eInfo

				if time.Now().Sub(eInfo.Time) < 1*time.Second {
					f.logger.Debug(fmt.Sprintf("too many events (%d) under 1s ... ignoring", eInfo.Counter))
					continue
				}

				//lstat, err := os.Lstat(event.Name)
				//if err != nil {
				//	f.logger.Error(err)
				//	return
				//}
				//f.eventMap[event.Name] = eventInfo{Time: time.Now(), FileInfo: lstat}
				// f.eventMap[event.Name] = eInfo

				//if eInfo.FileInfo != nil && lstat.Size() == eInfo.FileInfo.Size() {
				//	f.logger.Debug(fmt.Sprintf("%s has not changed", event.Name))
				//	continue
				//}

				if err := watcherFunc(Event(event), event.Name); err != nil {
					f.logger.Error(err)
					return
				}
				eInfo.Time = time.Now()
				eInfo.Counter = 0
				f.eventMap[event.Name] = eInfo

				tDiff := time.Now().Sub(t).Milliseconds()
				f.logger.Debug(fmt.Sprintf("[TIME TAKEN] to process watch loop: %dms", tDiff))
			}
		case err, ok := <-f.watcher.Errors:
			if !ok {
				return
			}
			f.logger.Error(err)
		}
	}
}

func (f fsnWatcher) RecursiveAdd(dirs ...string) error {
	for i := range dirs {
		if err := filepath.WalkDir(dirs[i], func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return f.watcher.Add(path)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (f fsnWatcher) Add(dir ...string) error {
	for i := range dir {
		err := f.watcher.Add(dir[i])
		if err != nil {
			log.Println(err)
			return err
		}
	}
	return nil
}

func (f fsnWatcher) Close() error {
	return f.watcher.Close()
}

type WatcherCtx struct {
	Logger           logging.Logger
	WatchExtensions  []string
	IgnoreExtensions []string
	ExcludeDirs      []string
}

func NewWatcher(ctx WatcherCtx) Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	return &fsnWatcher{
		watcher:          watcher,
		logger:           ctx.Logger,
		watchExtensions:  ctx.WatchExtensions,
		ignoreExtensions: ctx.IgnoreExtensions,
		excludeDirs:      ctx.ExcludeDirs,
	}
}
