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
	commands  []func(context.Context) *exec.Cmd

	interactive bool

	mu sync.Mutex

	kill func() error
}

type CmdExecutorArgs struct {
	Logger      *slog.Logger
	Commands    []func(context.Context) *exec.Cmd
	Interactive bool
}

func NewCmdExecutor(ctx context.Context, args CmdExecutorArgs) *CmdExecutor {
	if args.Logger == nil {
		args.Logger = slog.Default()
	}

	return &CmdExecutor{
		parentCtx:   ctx,
		logger:      args.Logger,
		commands:    args.Commands,
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

func killPID(pid int, logger ...*slog.Logger) error {
	var l *slog.Logger
	if len(logger) > 0 {
		l = logger[0]
	} else {
		l = slog.Default()
	}

	l.Debug("about to kill", "process", pid)
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		if err == syscall.ESRCH {
			return nil
		}
		l.Error("failed to kill, got", "err", err)
		return err
	}
	return nil
}

// Start implements Executor.
func (ex *CmdExecutor) Start() error {
	ex.mu.Lock()
	defer ex.mu.Unlock()
	for i := range ex.commands {
		if err := ex.parentCtx.Err(); err != nil {
			return err
		}

		ctx, cf := context.WithCancel(ex.parentCtx)
		defer cf()

		cmd := ex.commands[i](ctx)

		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if ex.interactive {
			cmd.Stdin = os.Stdin
			cmd.SysProcAttr.Foreground = true
		}

		if err := cmd.Start(); err != nil {
			return err
		}

		logger := ex.logger.With("pid", cmd.Process.Pid, "command", i+1)

		ex.kill = func() error {
			return killPID(cmd.Process.Pid, logger)
		}

		go func() {
			if err := cmd.Wait(); err != nil {
				logger.Debug("process finished (wait completed), got", "err", err)
			}
			cf()
		}()

		select {
		case <-ctx.Done():
			logger.Debug("process finished (context cancelled)")
		case <-ex.parentCtx.Done():
			logger.Debug("process finished (parent context cancelled)")
		}

		if ex.interactive {
			// Send SIGTERM to the interactive process, as user will see it on his screen
			proc, err := os.FindProcess(os.Getpid())
			if err != nil {
				return err
			}

			err = proc.Signal(syscall.SIGTERM)
			if err != nil {
				if err != syscall.ESRCH {
					logger.Error("failed to kill, got", "err", err)
					return err
				}
				return err
			}
		}

		if err := ex.kill(); err != nil {
			return err
		}

		logger.Debug("command fully executed and processed")
	}

	return nil
}

// Stop implements Executor.
func (ex *CmdExecutor) Stop() error {
	if ex.kill != nil {
		return ex.kill()
	}
	return nil
}

var _ Executor = (*CmdExecutor)(nil)
