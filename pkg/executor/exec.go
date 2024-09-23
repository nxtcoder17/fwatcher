package executor

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

type Executor struct {
	done      chan os.Signal
	logger    *slog.Logger
	isRunning bool

	newCmd func(context.Context) *exec.Cmd
	mu     sync.Mutex
}

type ExecutorArgs struct {
	Logger  *slog.Logger
	Command func(context.Context) *exec.Cmd
}

func NewExecutor(args ExecutorArgs) *Executor {
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGTERM)

	if args.Logger == nil {
		args.Logger = slog.Default()
	}

	return &Executor{
		done:   done,
		logger: args.Logger,
		newCmd: args.Command,
		mu:     sync.Mutex{},
	}
}

func (ex *Executor) Exec() error {
	ex.logger.Debug("[exec:pre] starting process")
	ex.mu.Lock()
	ex.isRunning = true

	defer func() {
		ex.isRunning = false
		ex.mu.Unlock()
	}()

	ex.logger.Debug("[exec] starting process")

	ctx, cf := context.WithCancel(context.TODO())
	defer cf()

	cmd := ex.newCmd(ctx)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		select {
		case <-ctx.Done():
			ex.logger.Debug("process terminated", "pid", cmd.Process.Pid)
		case <-ex.done:
			ex.logger.Debug("executor terminated", "pid", cmd.Process.Pid)
		}
		for len(ex.done) > 0 {
			<-ex.done
		}

		ex.logger.Debug("[exec] killing process", "pid", cmd.Process.Pid)
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			if err.Error() != "no such process" {
				ex.logger.Error("failed to kill process", "pid", cmd.Process.Pid, "err", err)
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		if strings.HasPrefix(err.Error(), "signal:") {
			ex.logger.Debug("wait terminated, received", "signal", err.Error())
		}
		ex.logger.Debug("while waiting, got", "err", err)
	}

	return nil
}

func (ex *Executor) Kill() {
	if ex.isRunning {
		ex.done <- os.Signal(syscall.SIGTERM)
	}
}
