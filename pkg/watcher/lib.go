package watcher

import (
	"context"
	"fmt"
	"sync"

	"github.com/nxtcoder17/fwatcher/pkg/executor"
)

func (f *Watcher) WatchAndExecute(ctx context.Context, executors []executor.Executor) error {
	var wg sync.WaitGroup

	l := len(executors)

	for i := 0; i < l-1; i++ {
		ex := executors[i]

		go func() {
			<-ctx.Done()
			ex.Stop()
		}()

		switch ex.(type) {
		case *executor.SSEExectuor:
			{
				wg.Add(1)
				go func() {
					defer wg.Done()
					ex.Start()
				}()
			}
		default:
			{
				if err := ex.Start(); err != nil {
					return err
				}

				// INFO: just for cleanup purposes
				if err := ex.Stop(); err != nil {
					return err
				}
			}
		}
	}

	ex := executors[l-1]

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := ex.Start(); err != nil {
			f.Logger.Error("starting command", "err", err)
		}
		f.Logger.Debug("final executor start finished")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		ex.Stop()
		f.Logger.Debug("2. context cancelled")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		f.Watch(ctx)
		f.Logger.Debug("3. watcher closed")
	}()

	counter := 0
	for event := range f.GetEvents() {
		f.Logger.Debug("received", "event", event)
		counter += 1

		f.Logger.Info(fmt.Sprintf("[RELOADING (%d)] due changes in %s", counter, event.Name))

		for i := range executors {
			executors[i].OnWatchEvent(executor.Event{Source: event.Name})
		}
	}

	wg.Wait()

	return nil
}
