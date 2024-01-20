package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	fswatcher "github.com/nxtcoder17/fwatcher/pkg/fs-watcher"
	"github.com/nxtcoder17/fwatcher/pkg/logging"
	"github.com/urfave/cli/v2"
)

type ProgramManager struct {
	done   chan os.Signal
	logger logging.Logger
}

func NewProgramManager(logger logging.Logger) *ProgramManager {
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGTERM)

	return &ProgramManager{
		done:   done,
		logger: logger,
	}
}

func (pm *ProgramManager) Exec(execCmd string) error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	cmd := exec.CommandContext(ctx, "sh", "-c", execCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return err
	}

	defer func() {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}()

	go func() {
		select {
		case <-pm.done:
			cancelFunc()
		}
	}()

	if err := cmd.Wait(); err != nil {
		if strings.HasPrefix(err.Error(), "signal:") {
			pm.logger.Debug(fmt.Sprintf("wait terminated as %s received", err.Error()))
		}
		return err
	}

	return nil
}

func main() {
	logger := logging.NewLogger()

	app := &cli.App{
		Name:  "fwatcher",
		Usage: "watches files in directories and operates on their changes",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:     "debug",
				Usage:    "toggles showing debug logs",
				Required: false,
				Value:    false,
			},
			&cli.StringFlag{
				Name:     "exec",
				Usage:    "specifies command to execute on file change",
				Required: true,
				// Aliases:  []string{"exec"},
			},
			&cli.PathFlag{
				Name:     "dir",
				Usage:    "directory to watch on",
				Required: false,
				Value: func() string {
					cwd, err := os.Getwd()
					if err != nil {
						panic(err)
					}
					return cwd
				}(),
				Aliases: []string{"d"},
				EnvVars: nil,
			},
			&cli.StringSliceFlag{
				Name:     "extensions",
				Usage:    "file extensions to watch on",
				Required: false,
				Aliases:  []string{"ext"},
			},
			&cli.StringSliceFlag{
				Name:     "ignore-extensions",
				Usage:    "file extensions to ignore watching on",
				Required: false,
				Aliases:  []string{"iext"},
			},
			&cli.StringSliceFlag{
				Name:     "exclude-dir",
				Usage:    "directory to exclude from watching",
				Required: false,
				Aliases:  []string{"exclude"},
			},
		},

		Action: func(ctx *cli.Context) error {
			logger.SetLogLevel(logging.InfoLevel)
			isDebugMode := ctx.Bool("debug")
			if isDebugMode {
				logger.SetLogLevel(logging.DebugLevel)
			}

			pm := NewProgramManager(logger)

			execCmd := ctx.String("exec")

			watchExt := ctx.StringSlice("extensions")
			ignoreExt := ctx.StringSlice("ignore-extensions")
			excludeDirs := ctx.StringSlice("exclude-dir")

			watcher := fswatcher.NewWatcher(fswatcher.WatcherCtx{
				Logger:           logger,
				WatchExtensions:  watchExt,
				IgnoreExtensions: ignoreExt,
				ExcludeDirs:      excludeDirs,
			})
			if err := watcher.RecursiveAdd(ctx.String("dir")); err != nil {
				panic(err)
			}

			go pm.Exec(execCmd)
			defer func() {
				pm.done <- syscall.SIGKILL
			}()

			go func() {
				select {
				case <-ctx.Done():
					print("ending ...")
					pm.done <- syscall.SIGKILL
					time.Sleep(100 * time.Millisecond)
					os.Exit(0)
				}
			}()

			watcher.WatchEvents(func(event fswatcher.Event, fp string) error {
				pm.done <- syscall.SIGKILL
				logger.Info(fmt.Sprintf("[RELOADING] due changes in %s", fp))
				go pm.Exec(execCmd)
				return nil
			})

			return nil
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()

	if err := app.RunContext(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
