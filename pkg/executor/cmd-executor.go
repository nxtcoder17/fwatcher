package executor

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

type CmdExecutor struct {
	logger    *slog.Logger
	parentCtx context.Context
	newCmd    func(context.Context) *exec.Cmd

	interactive bool

	mu    sync.Mutex
	abort func()
}

type CmdExecutorArgs struct {
	Logger      *slog.Logger
	Command     func(context.Context) *exec.Cmd
	Interactive bool
}

func NewCmdExecutor(ctx context.Context, args CmdExecutorArgs) *CmdExecutor {
	if args.Logger == nil {
		args.Logger = slog.Default()
	}

	return &CmdExecutor{
		parentCtx:   ctx,
		logger:      args.Logger.With("component", "cmd-executor"),
		newCmd:      args.Command,
		mu:          sync.Mutex{},
		interactive: args.Interactive,
	}
}

// OnWatchEvent implements Executor.
func (ex *CmdExecutor) OnWatchEvent(ev Event) error {
	ex.Stop()
	go ex.Start()
	return nil
}

// Start implements Executor.
func (ex *CmdExecutor) Start() error {
	ex.mu.Lock()
	ctx, cf := context.WithCancel(ex.parentCtx)
	ex.abort = cf
	ex.mu.Unlock()

	cmd := ex.newCmd(ctx)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if ex.interactive {
		cmd.Stdin = os.Stdin
		cmd.SysProcAttr.Foreground = true
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		ex.logger.Debug("process context done")
	case err := <-done:
		ex.logger.Debug("process wait completed, got", "err", err)
	}

	ex.logger.Debug("process", "pid", cmd.Process.Pid)

	if ex.interactive {
		// Send SIGTERM to the interactive process, as user will see it on his screen
		proc, err := os.FindProcess(os.Getpid())
		if err != nil {
			return err
		}

		err = proc.Signal(syscall.SIGTERM)
		if err != nil {
			if err != syscall.ESRCH {
				ex.logger.Error("failed to kill, got", "err", err)
				return err
			}
			return err
		}
	}

	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		if err == syscall.ESRCH {
			return nil
		}
		ex.logger.Error("failed to kill, got", "err", err)
		return err
	}

	return nil
}

// Stop implements Executor.
func (ex *CmdExecutor) Stop() error {
	ex.mu.Lock()
	if ex.abort != nil {
		ex.abort()
	}
	ex.mu.Unlock()
	return nil
}

var _ Executor = (*CmdExecutor)(nil)
