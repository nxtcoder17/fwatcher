package executor

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/nxtcoder17/go.pkgs/log"
)

type CommandGroup struct {
	Groups   []CommandGroup
	Commands []func(context.Context) *exec.Cmd
	Parallel bool
}

type CmdExecutor struct {
	logger    log.Logger
	parentCtx context.Context
	commands  []CommandGroup
	parallel  bool

	interactive bool

	mu sync.Mutex

	kill func() error
}

type CmdExecutorArgs struct {
	Logger      log.Logger
	Commands    []CommandGroup
	Parallel    bool
	Interactive bool
}

func NewCmdExecutor(ctx context.Context, args CmdExecutorArgs) *CmdExecutor {
	if args.Logger == nil {
		args.Logger = log.New(log.Options{})
	}

	return &CmdExecutor{
		parentCtx:   ctx,
		logger:      args.Logger,
		commands:    args.Commands,
		parallel:    args.Parallel,
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

func killPID(pid int, logger log.Logger) error {
	logger.Debug("about to kill", "process", pid)
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		if err == syscall.ESRCH {
			return nil
		}
		logger.Error(err, "failed to kill")
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
	if cmd == nil {
		return nil
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if ex.interactive {
		cmd.Stdin = os.Stdin
		cmd.SysProcAttr.Foreground = true
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	logger := ex.logger.With("pid", cmd.Process.Pid, "cmd", strings.Join(strings.Split(cmd.String(), " ")[2:], " "))

	logger.Debug("process started")

	pid := cmd.Process.Pid

	ex.kill = func() error {
		return killPID(pid, logger)
	}

	exitErr := make(chan error, 1)

	go func() {
		if err := cmd.Wait(); err != nil {
			exitErr <- err
			logger.Debug("process finished (wait completed), got", "err", err)
		}
		exitErr <- nil
	}()

	select {
	case <-ctx.Done():
		logger.Debug("process finished (context cancelled)", "reason", ctx.Err())

	case err := <-exitErr:
		if err == nil {
			// INFO: command exited with non-zero exit code
			logger.Debug("command SUCCESS", "exit.code", 0)
			return nil
		}

		logger.Error(err, "command failed")
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
				logger.Error(err, "failed to kill")
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

func (ex *CmdExecutor) execCommandGroup(cg CommandGroup) error {
	if cg.Parallel {
		var wg sync.WaitGroup

		ex.logger.Debug("PARALLEL", "len(cmds)", len(cg.Commands))
		for i := range cg.Commands {
			cmd := cg.Commands[i]
			wg.Add(1)
			go func() {
				defer wg.Done()

				ce := CmdExecutor{
					logger:      ex.logger.With("executor", i),
					parentCtx:   ex.parentCtx,
					interactive: ex.interactive,
					mu:          sync.Mutex{},
				}

				if err := ce.exec(cmd); err != nil {
					ex.logger.Error(err, "command failed")
					return
				}
			}()
		}

		for i := range cg.Groups {
			grp := cg.Groups[i]
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := ex.execCommandGroup(grp); err != nil {
					ex.logger.Error(err, "command group execution failed")
					return
				}
			}()
		}

		wg.Wait()
		return nil
	}

	for i := range cg.Commands {
		cmd := cg.Commands[i]
		if err := ex.exec(cmd); err != nil {
			return err
		}
	}

	for i := range cg.Groups {
		grp := cg.Groups[i]
		if err := ex.execCommandGroup(grp); err != nil {
			ex.logger.Error(err, "command group execution failed")
			return err
		}
	}

	return nil
}

// Start implements Executor.
func (ex *CmdExecutor) Start() error {
	ex.mu.Lock()
	defer ex.mu.Unlock()

	if ex.parallel {
		var wg sync.WaitGroup

		for i := range ex.commands {
			cg := ex.commands[i]
			wg.Add(1)
			go func() {
				defer wg.Done()

				ce := CmdExecutor{
					logger:      ex.logger.With("executor", i),
					parentCtx:   ex.parentCtx,
					interactive: ex.interactive,
					mu:          sync.Mutex{},
				}

				if err := ce.execCommandGroup(cg); err != nil {
					ex.logger.Error(err, "exec command group")
					return
				}
			}()
		}

		wg.Wait()
		return nil
	}

	for i := range ex.commands {
		cg := ex.commands[i]
		if err := ex.execCommandGroup(cg); err != nil {
			return err
		}
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
