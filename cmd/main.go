package main

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nxtcoder17/fwatcher/pkg/executor"
	"github.com/nxtcoder17/fwatcher/pkg/logging"
	"github.com/nxtcoder17/fwatcher/pkg/watcher"
	"github.com/urfave/cli/v3"
)

var (
	ProgramName string
	Version     string
)

func main() {
	cmd := &cli.Command{
		Name:                   ProgramName,
		UseShortOptionHandling: true,
		Usage:                  "simple tool to run commands on filesystem change events",
		ArgsUsage:              "<Command To Run>",
		Version:                Version,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name: "debug",
			},

			&cli.StringFlag{
				Name:    "command",
				Usage:   "[command to run]",
				Value:   "echo hi",
				Aliases: []string{"c"},
			},

			&cli.StringSliceFlag{
				Name:    "watch",
				Usage:   "[dir] (to watch) | -[dir] (to ignore)",
				Value:   []string{"."},
				Aliases: []string{"w"},
			},

			&cli.StringSliceFlag{
				Name:     "ext",
				Usage:    "[ext] (to watch) | -[ext] (to ignore)",
				Required: false,
				Aliases:  []string{"e"},
			},

			// &cli.StringSliceFlag{
			// 	Name:    "exclude",
			// 	Usage:   "exclude this directory",
			// 	Aliases: []string{"x"},
			// },

			&cli.StringSliceFlag{
				Name:    "ignore-list",
				Usage:   "disables ignoring from default ignore list",
				Value:   watcher.DefaultIgnoreList,
				Aliases: []string{"I"},
			},

			&cli.StringFlag{
				Name:  "cooldown",
				Usage: "cooldown duration",
				Value: "100ms",
			},

			&cli.BoolFlag{
				Name:  "interactive",
				Usage: "interactive mode, with stdin",
			},

			&cli.BoolFlag{
				Name:  "sse",
				Usage: "run watcher in sse mode",
			},

			&cli.StringFlag{
				Name:        "sse-addr",
				HideDefault: false,
				Usage:       "run watcher in sse mode",
				Sources:     cli.ValueSourceChain{},
				Value:       ":12345",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			logger := logging.NewSlogLogger(logging.SlogOptions{
				ShowTimestamp:      false,
				ShowCaller:         false,
				ShowDebugLogs:      c.Bool("debug"),
				SetAsDefaultLogger: true,
			})

			if c.NArg() == 0 {
				return c.Command("help").Action(ctx, c)
			}

			var watchDirs, excludeDirs []string

			for _, d := range c.StringSlice("watch") {
				if strings.HasPrefix(d, "-") {
					// INFO: needs to be excluded
					excludeDirs = append(excludeDirs, d[1:])
					continue
				}
				watchDirs = append(watchDirs, d)
			}

			var watchExtensions, ignoreExtensions []string
			for _, ext := range c.StringSlice("ext") {
				if strings.HasPrefix(ext, "-") {
					// INFO: needs to be excluded
					ignoreExtensions = append(ignoreExtensions, ext[1:])
					continue
				}
				watchExtensions = append(watchExtensions, ext)
			}

			cooldown, err := time.ParseDuration(c.String("cooldown"))
			if err != nil {
				panic(err)
			}

			args := watcher.WatcherArgs{
				Logger: logger,

				WatchDirs:  watchDirs,
				IgnoreDirs: excludeDirs,

				WatchExtensions:  watchExtensions,
				IgnoreExtensions: ignoreExtensions,
				CooldownDuration: &cooldown,

				IgnoreList: c.StringSlice("ignore-list"),
			}

			w, err := watcher.NewWatcher(ctx, args)
			if err != nil {
				panic(err)
			}

			var executors []executor.Executor

			if c.Bool("sse") {
				sseAddr := c.String("sse-addr")
				executors = append(executors, executor.NewSSEExecutor(executor.SSEExecutorArgs{Addr: sseAddr}))
			}

			if c.NArg() > 0 {
				execCmd := c.Args().First()
				execArgs := c.Args().Tail()
				executors = append(executors, executor.NewCmdExecutor(ctx, executor.CmdExecutorArgs{
					Logger:      logger,
					Interactive: c.Bool("interactive"),
					Command: func(context.Context) *exec.Cmd {
						cmd := exec.CommandContext(ctx, execCmd, execArgs...)
						cmd.Stdout = os.Stdout
						cmd.Stderr = os.Stderr
						cmd.Stdin = os.Stdin
						return cmd
					},
				}))
			}

			if err := w.WatchAndExecute(ctx, executors); err != nil {
				return err
			}

			return nil
		},
	}

	ctx, stop := signal.NotifyContext(context.TODO(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGABRT)
	defer stop()

	if err := cmd.Run(ctx, os.Args); err != nil {
		panic(err)
	}
	os.Exit(0)
}
