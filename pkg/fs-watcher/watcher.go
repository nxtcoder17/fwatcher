package fs_watcher

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/nxtcoder17/fwatcher/pkg/logging"
	"io/fs"
	"log"
	"os"
	"path/filepath"
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
}

type fsnWatcher struct {
	watcher  *fsnotify.Watcher
	eventMap map[string]eventInfo
	logger   logging.Logger
}

type Event string

var (
	Create = Event(fsnotify.Create.String())
	Delete = Event(fsnotify.Remove.String())
	Update = Event(fsnotify.Write.String())
	Rename = Event(fsnotify.Rename.String())
	Chmod  = Event(fsnotify.Chmod.String())
)

func (f fsnWatcher) WatchEvents(watcherFunc func(event Event, fp string) error) {
	f.eventMap = map[string]eventInfo{}
	for {
		select {
		case event, ok := <-f.watcher.Events:
			{
				t := time.Now()
				if !ok {
					return
				}

				f.logger.Debug(fmt.Sprintf("event %+v received", event))

				eInfo, ok := f.eventMap[event.Name]
				if !ok {
					eInfo = eventInfo{Time: time.Time{}, FileInfo: nil}
				}

				if time.Now().Sub(eInfo.Time) < 1*time.Second {
					f.logger.Debug("too many events ... ignoring")
				}

				lstat, err := os.Lstat(event.Name)
				if err != nil {
					f.logger.Error(err)
					return
				}
				f.eventMap[event.Name] = eventInfo{Time: time.Now(), FileInfo: lstat}

				if eInfo.FileInfo != nil && lstat.Size() == eInfo.FileInfo.Size() {
					f.logger.Debug(fmt.Sprintf("%s has not changed", event.Name))
					continue
				}

				if err := watcherFunc(Event(event.String()), event.Name); err != nil {
					f.logger.Error(err)
					return
				}

				tDiff := time.Now().Sub(t).Milliseconds()
				f.logger.Debug(fmt.Sprintf("time taken: %dms", tDiff))
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
	Logger logging.Logger
}

func NewWatcher(ctx WatcherCtx) Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	return &fsnWatcher{watcher: watcher, logger: ctx.Logger}
}
