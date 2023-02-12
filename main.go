package main

import (
	"context"
	"fmt"
	fswatcher "github.com/nxtcoder17/fwatcher/pkg/fs-watcher"
	"github.com/nxtcoder17/fwatcher/pkg/logging"
	"github.com/urfave/cli/v2"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type ProgramCtx struct {
	context.Context
	Logger logging.Logger
}

func runProgram(ctx ProgramCtx, execCmd string) error {
	cmd := exec.CommandContext(ctx, "bash", "-c", execCmd)
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

	if err := cmd.Wait(); err != nil {
		if err.Error() != "signal: killed" {
			ctx.Logger.Error(err)
			return err
		}
	}

	return nil
}

func main() {
	logger := logging.NewLogger()
	app := &cli.App{
		Name:  "fwatcher",
		Usage: "watches files in directories and operates on their changes",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "exec",
				Usage:    "specifies command to execute on file change",
				Required: true,
				Aliases:  []string{"e"},
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
		},
		Action: func(ctx *cli.Context) error {
			execCmd := ctx.String("exec")

			watcher := fswatcher.NewWatcher(fswatcher.WatcherCtx{Logger: logger})
			if err := watcher.RecursiveAdd(ctx.String("dir")); err != nil {
				panic(err)
			}

			cCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			commandCtx := ProgramCtx{
				Context: cCtx,
				Logger:  logger,
			}
			go runProgram(commandCtx, execCmd)

			watcher.WatchEvents(func(event fswatcher.Event, fp string) error {
				logger.Info(fmt.Sprintf("[RELOADING] due changes in %s", fp))
				cancel()
				time.Sleep(100 * time.Millisecond)
				if commandCtx.Err() != nil {
					cCtx, cancel = context.WithCancel(context.Background())
					commandCtx = ProgramCtx{
						Context: cCtx,
						Logger:  logger,
					}
				}
				go runProgram(commandCtx, execCmd)
				return nil
			})

			// Block main goroutine forever.
			<-make(chan struct{})
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
