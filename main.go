package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/nxtcoder17/fwatcher/pkg/executor"
	"github.com/nxtcoder17/fwatcher/pkg/logging"
	fswatcher "github.com/nxtcoder17/fwatcher/pkg/watcher"
	"github.com/urfave/cli/v2"
)

var Version string

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()

	app := &cli.App{
		Name:           "fwatcher",
		Usage:          "watches files in directories and operates on their changes",
		Version:        Version,
		DefaultCommand: "help",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:     "debug",
				Usage:    "toggles showing debug logs",
				Required: false,
				Value:    false,
			},
			&cli.StringFlag{
				Name:     "command",
				Usage:    "specifies command to execute on file change",
				Required: false,
				Aliases:  []string{"c"},
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
				Name:     "ignore-suffixes",
				Usage:    "files suffixes to ignore",
				Required: false,
				Aliases:  []string{"i"},
			},
			&cli.StringSliceFlag{
				Name:     "exclude-dir",
				Usage:    "directory to exclude from watching",
				Required: false,
				Aliases:  []string{"x", "e"},
			},
			&cli.BoolFlag{
				Name:     "no-default-ignore",
				Usage:    "disables ignoring from default ignore list",
				Required: false,
				Aliases:  []string{"I"},
				Value:    false,
			},
		},

		Action: func(cctx *cli.Context) error {
			logger := logging.NewSlogLogger(logging.SlogOptions{
				ShowTimestamp:      false,
				ShowCaller:         false,
				ShowDebugLogs:      cctx.Bool("debug"),
				SetAsDefaultLogger: true,
			})

			var execCmd string
			var execArgs []string

			if cctx.Args().Len() == 0 && cctx.String("command") == "" {
				return fmt.Errorf("no command specified")
			}

			if cctx.String("command") != "" {
				s := strings.SplitN(cctx.String("command"), " ", 2)

				switch len(s) {
				case 0:
					return fmt.Errorf("invalid command")
				case 1:
					execCmd = s[0]
					execArgs = nil
				case 2:
					execCmd = s[0]
					execArgs = strings.Split(s[1], " ")
				}
			} else {
				logger.Debug("no command specified, using args")
				if cctx.Args().Len() == 0 {
					return fmt.Errorf("no command specified")
				}

				execCmd = cctx.Args().First()
				execArgs = cctx.Args().Tail()
			}

			ex := executor.NewExecutor(executor.ExecutorArgs{
				Logger: logger,
				Command: func(context.Context) *exec.Cmd {
					cmd := exec.Command(execCmd, execArgs...)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					cmd.Stdin = os.Stdin
					return cmd
				},
			})

			watcher, err := fswatcher.NewWatcher(fswatcher.WatcherArgs{
				Logger:               logger,
				IgnoreSuffixes:       cctx.StringSlice("ignore-suffixes"),
				ExcludeDirs:          cctx.StringSlice("exclude-dir"),
				UseDefaultIgnoreList: !cctx.Bool("no-global-ignore"),
			})
			if err != nil {
				panic(err)
			}

			if err := watcher.RecursiveAdd(cctx.String("dir")); err != nil {
				panic(err)
			}

			go ex.Exec()

			go func() {
				<-ctx.Done()
				logger.Debug("fwatcher is closing ...")
				<-time.After(200 * time.Millisecond)
				os.Exit(0)
			}()

			watcher.WatchEvents(func(event fswatcher.Event, fp string) error {
				relPath, err := filepath.Rel(cctx.String("dir"), fp)
				if err != nil {
					return err
				}
				logger.Info(fmt.Sprintf("[RELOADING] due changes in %s", relPath))
				ex.Kill()
				<-time.After(100 * time.Millisecond)
				go ex.Exec()
				return nil
			})

			return nil
		},
	}

	if err := app.RunContext(ctx, os.Args); err != nil {
		os.Exit(1)
	}
}
