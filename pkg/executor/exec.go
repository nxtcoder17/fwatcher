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
	done   chan os.Signal
	logger *slog.Logger

	mu      sync.Mutex
	running *os.Process

	isInteractive bool

	newCmd func(context.Context) *exec.Cmd
}

type ExecutorArgs struct {
	Logger        *slog.Logger
	Command       func(context.Context) *exec.Cmd
	IsInteractive bool
}

func NewExecutor(args ExecutorArgs) *Executor {
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	if args.Logger == nil {
		args.Logger = slog.Default()
	}

	return &Executor{
		done:          done,
		logger:        args.Logger,
		isInteractive: args.IsInteractive,
		newCmd:        args.Command,
		mu:            sync.Mutex{},
	}
}

// ctx is process context
func (ex *Executor) handleExits(ctx context.Context, cf context.CancelFunc) {
	if ex.running == nil {
		return
	}

	defer func() {
		ex.running = nil
		cf()
	}()

	select {
	case <-ctx.Done():
		// INFO: process exited
		ex.logger.Debug("process terminated", "pid", ex.running.Pid)
		// if ex.running != nil {
		// }

		if ex.isInteractive {
			os.Exit(0)
		}

	case sig := <-ex.done:
		// INFO: fwatcher exited
		ex.logger.Debug("executor terminated", "received-signal", sig)
		cf()
	}

	for len(ex.done) > 0 {
		<-ex.done
	}

	if ex.isInteractive {
		// INFO: interactive, only kill the process remains
		ex.logger.Debug("[exec] killing process", "pid", ex.running.Pid)
		ex.running.Signal(syscall.SIGKILL)
		return
	}

	// INFO: non-interactive killing the entire child tree
	ex.logger.Debug("[exec] killing process", "pid", ex.running.Pid)
	if err := syscall.Kill(-ex.running.Pid, syscall.SIGKILL); err != nil {
		if err.Error() != "no such process" {
			ex.logger.Error("failed to kill process", "pid", ex.running.Pid, "err", err)
		}
	}
}

// DO NOT USE: does not work yet.
func (ex *Executor) Exec() error {
	ex.logger.Debug("[exec:pre] starting process")
	ex.mu.Lock()

	defer ex.mu.Unlock()

	ctx, cf := context.WithCancel(context.TODO())
	defer cf()

	cmd := ex.newCmd(ctx)
	if !ex.isInteractive {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	ex.running = cmd.Process

	go ex.handleExits(ctx, cf)

	// defer func() { ex.running = nil }()
	// go func() {
	// 	select {
	// 	case <-ctx.Done():
	// 		// INFO: process exited
	// 		ex.logger.Debug("process terminated", "pid", cmd.Process.Pid)
	// 		<-time.After(1 * time.Second)
	// 		if ex.isInteractive {
	// 			os.Exit(0)
	// 		}
	//
	// 	case sig := <-ex.done:
	// 		// INFO: fwatcher exited
	// 		ex.logger.Debug("executor terminated", "received-signal", sig)
	// 		cf()
	// 	}
	//
	// 	for len(ex.done) > 0 {
	// 		<-ex.done
	// 	}
	//
	// 	if cmd.Process != nil {
	// 		ex.logger.Debug("[exec] killing process", "pid", cmd.Process.Pid)
	// 		cmd.Process.Signal(syscall.SIGKILL)
	// 	}
	//
	// 	ex.logger.Debug("[exec] killing process", "pid", cmd.Process.Pid)
	// 	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
	// 		if err.Error() != "no such process" {
	// 			ex.logger.Error("failed to kill process", "pid", cmd.Process.Pid, "err", err)
	// 		}
	// 	}
	// }()

	// ex.logger.Debug("[exec:post] process running")
	if err := cmd.Wait(); err != nil {
		ex.logger.Debug("cmd terminated with", "err", err)
		err2, ok := err.(*exec.ExitError)
		if ok {
			ex.logger.Debug("cmd terminated with", "exit-code", err2.ExitCode())
		}

		if strings.HasPrefix(err.Error(), "signal:") {
			ex.logger.Debug("wait terminated, received", "signal", err.Error())
		}
		ex.logger.Debug("while waiting, got", "err", err)
	}
	ex.logger.Debug("[exec] killed process", "pid", cmd.Process.Pid)

	return nil
}

func (ex *Executor) Kill() {
	if ex.running != nil {
		ex.done <- os.Signal(syscall.SIGTERM)
	}
}
