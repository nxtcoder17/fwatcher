package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/nxtcoder17/fwatcher/pkg/executor"
)

func (f *Watcher) WatchAndExecute(ctx context.Context, executors []executor.Executor) error {
	var wg sync.WaitGroup

	for _i := range executors {
		i := _i
		ex := executors[i]

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ex.Start(); err != nil {
				f.Logger.Error("got", "err", err)
			}
			f.Logger.Debug("1. executor start finished", "executor", i)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-ctx.Done()
			ex.Stop()
			f.Logger.Debug("2. passed context is DONE", "executor", i)
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		f.Watch(ctx)
	}()

	pwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	counter := 0
	for event := range f.GetEvents() {
		f.Logger.Debug("received", "event", event)
		relPath, err := filepath.Rel(pwd, event.Name)
		if err != nil {
			return err
		}
		counter += 1
		f.Logger.Info(fmt.Sprintf("[RELOADING (%d)] due changes in %s", counter, relPath))

		for i := range executors {
			executors[i].OnWatchEvent(executor.Event{Source: event.Name})
		}
	}

	wg.Wait()

	return nil
}
