package executor

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"testing"
)

func Test_Exectuor_Start(t *testing.T) {
	newCmd := func(stdout io.Writer, cmd string, args ...string) func(c context.Context) *exec.Cmd {
		return func(c context.Context) *exec.Cmd {
			cmd := exec.CommandContext(c, cmd, args...)
			cmd.Stdout = stdout
			return cmd
		}
	}

	tests := []struct {
		name     string
		commands func(stdout io.Writer) []func(c context.Context) *exec.Cmd
		output   []string
	}{
		// TODO: add your tests
		{
			name: "1. with single command",
			commands: func(stdout io.Writer) []func(c context.Context) *exec.Cmd {
				return []func(c context.Context) *exec.Cmd{
					newCmd(stdout, "echo", "hi"),
				}
			},
			output: []string{
				"hi",
			},
		},
		{
			name: "2. testing",
			commands: func(stdout io.Writer) []func(c context.Context) *exec.Cmd {
				return []func(c context.Context) *exec.Cmd{
					newCmd(stdout, "echo", "hi"),
					newCmd(stdout, "echo", "hello"),
				}
			},
			output: []string{
				"hi",
				"hello",
			},
		},
	}

	logger := slog.Default()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := new(bytes.Buffer)
			ex := NewCmdExecutor(context.TODO(), CmdExecutorArgs{
				Logger:      logger,
				Commands:    tt.commands(b),
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
