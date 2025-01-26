package executor

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

// type CommandGroup struct {
// 	Commands   []func(context.Context) *exec.Cmd
// 	Parallel   bool
// 	Sequential bool
// }

type CmdExecutor struct {
	logger    *slog.Logger
	parentCtx context.Context
	commands  []func(context.Context) *exec.Cmd

	interactive bool

	mu sync.Mutex

	kill func() error

	Parallel []ParallelCommands
}

type ParallelCommands struct {
	Index int
	Len   int
}

type CmdExecutorArgs struct {
	Logger      *slog.Logger
	Commands    []func(context.Context) *exec.Cmd
	Interactive bool
	Parallel    []ParallelCommands
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
		Parallel:    args.Parallel,
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

func (ex *CmdExecutor) exec(newCmd func(context.Context) *exec.Cmd) error {
	if err := ex.parentCtx.Err(); err != nil {
		return err
	}

	ctx, cf := context.WithCancel(ex.parentCtx)
	defer cf()

	cmd := newCmd(ctx)

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if ex.interactive {
		cmd.Stdin = os.Stdin
		cmd.SysProcAttr.Foreground = true
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	logger := ex.logger.With("pid", cmd.Process.Pid, "command", cmd.String())

	ex.kill = func() error {
		return killPID(cmd.Process.Pid, logger)
	}

	exitErr := make(chan error, 1)

	go func() {
		if err := cmd.Wait(); err != nil {
			exitErr <- err
			logger.Debug("process finished (wait completed), got", "err", err)
		}
		cf()
	}()

	select {
	case <-ctx.Done():
		logger.Debug("process finished (context cancelled)")
	case err := <-exitErr:
		if exitErr, ok := err.(*exec.ExitError); ok {
			logger.Debug("process finished", "exit.code", exitErr.ExitCode())
			if exitErr.ExitCode() != 0 {
				return err
			}
		}
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
	return nil
}

// Start implements Executor.
func (ex *CmdExecutor) Start() error {
	ex.mu.Lock()
	defer ex.mu.Unlock()

	var wg sync.WaitGroup

	for i := 0; i < len(ex.commands); i++ {
		newCmd := ex.commands[i]

		ex.logger.Info("HELLO", "idx", i, "ex.parallel", ex.Parallel)
		isParallel := false

		for _, p := range ex.Parallel {
			if p.Index == i {
				isParallel = true
				for k := i; k <= i+p.Len; k++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						if err := ex.exec(newCmd); err != nil {
							ex.logger.Info("executing, got", "err", err)
							// handle error
						}
					}()
				}

				i = i + p.Len - 1
			}
			break
		}

		if isParallel {
			continue
		}

		// if ex.Parallel {
		// 	wg.Add(1)
		// 	go func() {
		// 		defer wg.Add(1)
		// 		if err := ex.exec(newCmd); err != nil {
		// 			// handle error
		// 		}
		// 	}()
		// 	continue
		// }

		if err := ex.exec(newCmd); err != nil {
			ex.logger.Error("cmd failed with", "err", err)
			return err
		}
	}

	if len(ex.Parallel) > 0 {
		wg.Wait()
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
