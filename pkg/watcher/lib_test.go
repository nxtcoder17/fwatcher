package watcher

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/nxtcoder17/fwatcher/pkg/executor"
)

func Test_Watcher_WatchAndExecute(t *testing.T) {
	logLevel := slog.LevelInfo
	if os.Getenv("DEBUG") == "true" {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))

	slog.SetDefault(logger)

	newCmd := func(stdout io.Writer, cmd string, args ...string) func(c context.Context) *exec.Cmd {
		return func(c context.Context) *exec.Cmd {
			cmd := exec.CommandContext(c, cmd, args...)
			cmd.Stdout = stdout
			return cmd
		}
	}

	tests := []struct {
		name       string
		sendEvents func(ch chan Event)
		executors  func(ctx context.Context, stdout io.Writer) []executor.Executor
		want       []string
	}{
		{
			name: "1. single executor, single command",
			sendEvents: func(ch chan Event) {
			},

			executors: func(ctx context.Context, stdout io.Writer) []executor.Executor {
				return []executor.Executor{
					executor.NewCmdExecutor(ctx, executor.CmdExecutorArgs{
						Logger: logger,
						Commands: []func(context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "hi"),
						},
						Interactive: false,
					}),
				}
			},
			want: []string{
				"hi",
			},
		},

		{
			name: "2. single executor, multiple commands",
			sendEvents: func(ch chan Event) {
			},

			executors: func(ctx context.Context, stdout io.Writer) []executor.Executor {
				return []executor.Executor{
					executor.NewCmdExecutor(ctx, executor.CmdExecutorArgs{
						Logger: logger,
						Commands: []func(context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "hi"),
							newCmd(stdout, "echo", "hello"),
						},
						Interactive: false,
					}),
				}
			},
			want: []string{
				"hi",
				"hello",
			},
		},

		{
			name: "3. multiple executor, single command each",
			sendEvents: func(ch chan Event) {
			},

			executors: func(ctx context.Context, stdout io.Writer) []executor.Executor {
				return []executor.Executor{
					executor.NewCmdExecutor(ctx, executor.CmdExecutorArgs{
						Logger: logger,
						Commands: []func(context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "hi"),
						},
						Interactive: false,
					}),

					executor.NewCmdExecutor(ctx, executor.CmdExecutorArgs{
						Logger: logger,
						Commands: []func(context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "hello"),
						},
						Interactive: false,
					}),
				}
			},
			want: []string{
				"hi",
				"hello",
			},
		},

		{
			name: "4. multiple executor, multiple commands each",
			sendEvents: func(ch chan Event) {
			},

			executors: func(ctx context.Context, stdout io.Writer) []executor.Executor {
				return []executor.Executor{
					executor.NewCmdExecutor(ctx, executor.CmdExecutorArgs{
						Logger: logger,
						Commands: []func(context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "hi"),
							newCmd(stdout, "echo", "hello"),
						},
						Interactive: false,
					}),

					executor.NewCmdExecutor(ctx, executor.CmdExecutorArgs{
						Logger: logger,
						Commands: []func(context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "no hi"),
							newCmd(stdout, "echo", "no hello"),
						},
						Interactive: false,
					}),
				}
			},
			want: []string{
				"hi",
				"hello",
				"no hi",
				"no hello",
			},
		},

		{
			name: "5. single executor, single command, single change event",
			sendEvents: func(ch chan Event) {
				<-time.After(20 * time.Millisecond)
				ch <- Event{Name: "sample", Op: fsnotify.Create}
			},

			executors: func(ctx context.Context, stdout io.Writer) []executor.Executor {
				return []executor.Executor{
					executor.NewCmdExecutor(ctx, executor.CmdExecutorArgs{
						Logger: logger,
						Commands: []func(context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "hi"),
						},
						Interactive: false,
					}),
				}
			},
			want: []string{
				"hi",
				// this one after first event
				"hi",
			},
		},

		{
			name: "6. single executor, single command, multiple change events",
			sendEvents: func(ch chan Event) {
				<-time.After(20 * time.Millisecond)
				ch <- Event{Name: "sample", Op: fsnotify.Create}

				<-time.After(40 * time.Millisecond)
				ch <- Event{Name: "sample", Op: fsnotify.Create}
			},

			executors: func(ctx context.Context, stdout io.Writer) []executor.Executor {
				return []executor.Executor{
					executor.NewCmdExecutor(ctx, executor.CmdExecutorArgs{
						Logger: logger,
						Commands: []func(context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "hi"),
						},
						Interactive: false,
					}),
				}
			},
			want: []string{
				"hi",

				// this one after first event
				"hi",

				// this one after second event
				"hi",
			},
		},

		{
			name: "7. single executor, multiple commands, single change event",
			sendEvents: func(ch chan Event) {
				<-time.After(20 * time.Millisecond)
				ch <- Event{Name: "sample", Op: fsnotify.Create}
			},

			executors: func(ctx context.Context, stdout io.Writer) []executor.Executor {
				return []executor.Executor{
					executor.NewCmdExecutor(ctx, executor.CmdExecutorArgs{
						Logger: logger,
						Commands: []func(context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "hi"),
							newCmd(stdout, "echo", "hello"),
						},
						Interactive: false,
					}),
				}
			},
			want: []string{
				"hi",
				"hello",

				// after first change event
				"hi",
				"hello",
			},
		},

		{
			name: "8. single executor, multiple commands, multiple change events",
			sendEvents: func(ch chan Event) {
				<-time.After(20 * time.Millisecond)
				ch <- Event{Name: "sample", Op: fsnotify.Create}

				<-time.After(20 * time.Millisecond)
				ch <- Event{Name: "sample", Op: fsnotify.Create}
			},

			executors: func(ctx context.Context, stdout io.Writer) []executor.Executor {
				return []executor.Executor{
					executor.NewCmdExecutor(ctx, executor.CmdExecutorArgs{
						Logger: logger,
						Commands: []func(context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "hi"),
							newCmd(stdout, "echo", "hello"),
						},
						Interactive: false,
					}),
				}
			},
			want: []string{
				"hi",
				"hello",

				// after first change event
				"hi",
				"hello",

				// after second change event
				"hi",
				"hello",
			},
		},

		{
			name: "9. multiple executor, single command, single change event",
			sendEvents: func(ch chan Event) {
				<-time.After(20 * time.Millisecond)
				ch <- Event{Name: "sample", Op: fsnotify.Create}
			},

			executors: func(ctx context.Context, stdout io.Writer) []executor.Executor {
				return []executor.Executor{
					executor.NewCmdExecutor(ctx, executor.CmdExecutorArgs{
						Logger: logger,
						Commands: []func(context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "hi"),
						},
						Interactive: false,
					}),
				}
			},
			want: []string{
				"hi",
				// after first change event
				"hi",
			},
		},

		{
			name: "10. multiple executor, single command, multiple change event",
			sendEvents: func(ch chan Event) {
				<-time.After(20 * time.Millisecond)
				ch <- Event{Name: "sample", Op: fsnotify.Create}

				<-time.After(20 * time.Millisecond)
				ch <- Event{Name: "sample", Op: fsnotify.Create}
			},

			executors: func(ctx context.Context, stdout io.Writer) []executor.Executor {
				return []executor.Executor{
					executor.NewCmdExecutor(ctx, executor.CmdExecutorArgs{
						Logger: logger,
						Commands: []func(context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "hi"),
						},
						Interactive: false,
					}),
				}
			},
			want: []string{
				"hi",

				// after first change event
				"hi",

				// after second change event
				"hi",
			},
		},

		{
			name: "11. multiple executor, multiple commands, single change event",
			sendEvents: func(ch chan Event) {
				<-time.After(20 * time.Millisecond)
				ch <- Event{Name: "sample", Op: fsnotify.Create}

				<-time.After(20 * time.Millisecond)
				ch <- Event{Name: "sample", Op: fsnotify.Create}
			},

			executors: func(ctx context.Context, stdout io.Writer) []executor.Executor {
				return []executor.Executor{
					executor.NewCmdExecutor(ctx, executor.CmdExecutorArgs{
						Logger: logger,
						Commands: []func(context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "hi"),
							newCmd(stdout, "echo", "hello"),
						},
						Interactive: false,
					}),
				}
			},
			want: []string{
				"hi",
				"hello",

				// after first change event
				"hi",
				"hello",

				// after second change event
				"hi",
				"hello",
			},
		},

		{
			name: "12. multiple executor, multiple commands, multiple change events",
			sendEvents: func(ch chan Event) {
				<-time.After(20 * time.Millisecond)
				ch <- Event{Name: "sample", Op: fsnotify.Create}
			},

			executors: func(ctx context.Context, stdout io.Writer) []executor.Executor {
				return []executor.Executor{
					executor.NewCmdExecutor(ctx, executor.CmdExecutorArgs{
						Logger: logger,
						Commands: []func(context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "hi"),
							newCmd(stdout, "echo", "hello"),
						},
						Interactive: false,
					}),
				}
			},
			want: []string{
				"hi",
				"hello",

				// after first change event
				"hi",
				"hello",
			},
		},

		{
			name: "13. multiple executor with SSE, multiple commands, multiple change events",
			sendEvents: func(ch chan Event) {
				<-time.After(20 * time.Millisecond)
				ch <- Event{Name: "sample", Op: fsnotify.Create}
			},

			executors: func(ctx context.Context, stdout io.Writer) []executor.Executor {
				return []executor.Executor{
					executor.NewSSEExecutor(executor.SSEExecutorArgs{
						Addr:   ":8919",
						Logger: logger,
					}),

					executor.NewCmdExecutor(ctx, executor.CmdExecutorArgs{
						Logger: logger,
						Commands: []func(context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "hi"),
							newCmd(stdout, "echo", "hello"),
						},
						Interactive: false,
					}),
				}
			},
			want: []string{
				"hi",
				"hello",

				// after first change event
				"hi",
				"hello",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventCh := make(chan Event)
			go tt.sendEvents(eventCh)

			b := new(bytes.Buffer)

			w, _ := fsnotify.NewWatcher()

			watcher := Watcher{
				watcher:          w,
				Logger:           logger,
				cooldownDuration: 5 * time.Millisecond,
				eventsCh:         eventCh,
			}

			ctx, cf := context.WithTimeout(context.TODO(), 100*time.Millisecond)
			// ctx, cf := context.WithCancel(context.TODO())
			defer cf()

			executors := tt.executors(ctx, b)

			if err := watcher.WatchAndExecute(ctx, executors); err != nil {
				t.Error(err)
			}

			want := strings.Join(tt.want, "\n")
			got := strings.TrimSpace(b.String())

			if got != want {
				t.Errorf("FAILED (%s)\n\t got: %s\n\twant: %s\n", tt.name, got, want)
			}
		})
	}
}
