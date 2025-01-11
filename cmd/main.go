package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nxtcoder17/fwatcher/pkg/executor"
	fn "github.com/nxtcoder17/fwatcher/pkg/functions"
	"github.com/nxtcoder17/fwatcher/pkg/logging"
	"github.com/nxtcoder17/fwatcher/pkg/watcher"
	"github.com/urfave/cli/v3"
)

var (
	ProgramName string
	Version     string
)

// DefaultIgnoreList is list of directories that are mostly ignored
var DefaultIgnoreList = []string{
	".git", ".svn", ".hg", // version control
	".idea", ".vscode", // IDEs
	".direnv",      // direnv nix
	"node_modules", // node
	".DS_Store",    // macOS
	".log",         // logs
}

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
				Value:   DefaultIgnoreList,
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

			var ex executor.Executor

			switch {
			case c.Bool("sse"):
				{
					sseAddr := c.String("sse-addr")
					ex = executor.NewSSEExecutor(executor.SSEExecutorArgs{Addr: sseAddr})
					logger.Info("HELLo world")
				}
			default:
				{
					execCmd := c.Args().First()
					execArgs := c.Args().Tail()
					ex = executor.NewCmdExecutor(ctx, executor.CmdExecutorArgs{
						Logger:      logger,
						Interactive: c.Bool("interactive"),
						Command: func(context.Context) *exec.Cmd {
							cmd := exec.CommandContext(ctx, execCmd, execArgs...)
							cmd.Stdout = os.Stdout
							cmd.Stderr = os.Stderr
							cmd.Stdin = os.Stdin
							return cmd
						},
						// IsInteractive: true,
					})
				}
			}

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := ex.Start(); err != nil {
					slog.Error("got", "err", err)
				}
				logger.Debug("1. start-job finished")
			}()

			counter := 0
			pwd := fn.Must(os.Getwd())

			wg.Add(1)
			go func() {
				defer wg.Done()
				w.Watch(ctx)
				logger.Debug("2. watch context closed")
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				<-ctx.Done()
				ex.Stop()
				logger.Debug("3. killed signal processed")
			}()

			for event := range w.GetEvents() {
				logger.Debug("received", "event", event)
				relPath, err := filepath.Rel(pwd, event.Name)
				if err != nil {
					return err
				}
				counter += 1
				logger.Info(fmt.Sprintf("[RELOADING (%d)] due changes in %s", counter, relPath))

				ex.OnWatchEvent(executor.Event{Source: event.Name})
			}

			// logger.Debug("stopping executor")
			// if err := ex.Stop(); err != nil {
			// 	return err
			// }
			// logger.Info("stopped executor")

			wg.Wait()
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
