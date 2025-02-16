package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/nxtcoder17/go.pkgs/log"
)

type Writer struct {
	b *bytes.Buffer
	m sync.Mutex
}

func (w *Writer) Write(b []byte) (int, error) {
	w.m.Lock()
	defer w.m.Unlock()
	return w.b.Write(b)
}

func Test_Exectuor_Start(t *testing.T) {
	newCmd := func(stdout io.Writer, cmd string, args ...string) func(c context.Context) *exec.Cmd {
		return func(c context.Context) *exec.Cmd {
			args := []string{
				"-c",
				fmt.Sprintf("%s %s", cmd, strings.Join(args, " ")),
			}
			cmd := exec.CommandContext(c, "sh", args...)
			cmd.Stdout = stdout
			cmd.Stderr = os.Stderr
			return cmd
		}
	}

	tests := []struct {
		name          string
		commands      func(stdout io.Writer) []CommandGroup
		parallel      bool
		showDebugLogs bool
		output        []string
	}{
		// TODO: add your tests
		{
			name: "1. with single command",
			commands: func(stdout io.Writer) []CommandGroup {
				return []CommandGroup{
					{
						Commands: []func(c context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "hi"),
						},
					},
				}
			},

			output: []string{
				"hi",
			},
		},
		{
			name: "2. with multiple commands",
			commands: func(stdout io.Writer) []CommandGroup {
				return []CommandGroup{
					{
						Commands: []func(c context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "hi"),
							newCmd(stdout, "echo", "hello"),
						},
					},
				}
			},

			output: []string{
				"hi",
				"hello",
			},
		},
		{
			name: "3. with multiple command groups",
			commands: func(stdout io.Writer) []CommandGroup {
				return []CommandGroup{
					{
						Commands: []func(c context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "first:hi"),
							newCmd(stdout, "echo", "first:hello"),
						},
					},
					{
						Commands: []func(c context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "second:hi"),
							newCmd(stdout, "echo", "second:hello"),
						},
					},
				}
			},

			output: []string{
				"first:hi",
				"first:hello",
				"second:hi",
				"second:hello",
			},
		},

		{
			name: "4. with multiple command groups (sequential)",
			commands: func(stdout io.Writer) []CommandGroup {
				return []CommandGroup{
					{
						Commands: []func(c context.Context) *exec.Cmd{
							newCmd(stdout, "sleep", "1"),
							newCmd(stdout, "echo", "first:hello"),
						},
					},
					{
						Commands: []func(c context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "second:hi"),
							newCmd(stdout, "echo", "second:hello"),
						},
					},
				}
			},
			parallel: false,

			output: []string{
				"first:hello",
				"second:hi",
				"second:hello",
			},
		},

		{
			name: "5. with multiple command groups (parallel)",
			commands: func(stdout io.Writer) []CommandGroup {
				return []CommandGroup{
					{
						Commands: []func(c context.Context) *exec.Cmd{
							newCmd(stdout, "sleep", "2"),
							newCmd(stdout, "echo", "first:hello"),
						},
					},
					{
						Commands: []func(c context.Context) *exec.Cmd{
							newCmd(stdout, "echo", "second:hi"),
							newCmd(stdout, "echo", "second:hello"),
						},
					},
				}
			},
			parallel:      true,
			showDebugLogs: true,

			output: []string{
				"second:hi",
				"second:hello",
				"first:hello",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := new(bytes.Buffer)
			w := Writer{b: b, m: sync.Mutex{}}

			logger := log.New(log.Options{
				ShowDebugLogs: os.Getenv("DEBUG") == "true" || tt.showDebugLogs,
			})

			ex := NewCmdExecutor(context.TODO(), CmdExecutorArgs{
				Logger:      logger,
				Commands:    tt.commands(&w),
				Parallel:    tt.parallel,
				Interactive: false,
			})

			if err := ex.Start(); err != nil {
				t.Error(err)
			}

			want := strings.Join(tt.output, "\n")
			got := strings.TrimSpace(b.String())

			if got != want {
				t.Errorf("FAILED (%s)\n\t got: %s\n\twant: %s\n", tt.name, got, want)
			}
		})
	}
}
