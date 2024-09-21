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
	ctx    context.Context
	done   chan os.Signal
	logger *slog.Logger

	newCmd func(context.Context) *exec.Cmd
	mu     sync.Mutex
}

type ExecutorArgs struct {
	Logger  *slog.Logger
	Command func(context.Context) *exec.Cmd
}

func NewExecutor(ctx context.Context, args ExecutorArgs) *Executor {
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGTERM)

	if args.Logger == nil {
		args.Logger = slog.Default()
	}

	return &Executor{
		done:   done,
		logger: args.Logger,
		ctx:    ctx,
		newCmd: args.Command,
		mu:     sync.Mutex{},
	}
}

func (ex *Executor) Exec() error {
	ex.mu.Lock()
	defer ex.mu.Unlock()

	ex.logger.Debug("[exec] starting process")

	ctx, cf := context.WithCancel(ex.ctx)
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
		ex.logger.Debug("[exec] killing process", "pid", cmd.Process.Pid)
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			ex.logger.Error("failed to kill process", "pid", cmd.Process.Pid, "err", err)
		}
	}()

	if err := cmd.Wait(); err != nil {
		if strings.HasPrefix(err.Error(), "signal:") {
			ex.logger.Debug("wait terminated, received", "signal", err.Error())
		}
		return err
	}

	return nil
}

func (ex *Executor) Kill() {
	ex.done <- os.Signal(syscall.SIGTERM)
}
