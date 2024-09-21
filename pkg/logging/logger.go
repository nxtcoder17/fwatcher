package logging

import (
	"io"
	"log/slog"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

type SlogOptions struct {
	Writer io.Writer
	Prefix string

	ShowTimestamp bool
	ShowCaller    bool
	ShowDebugLogs bool

	SetAsDefaultLogger bool
}

func NewSlogLogger(opts SlogOptions) *slog.Logger {
	// INFO: force colored output, otherwise honor the env-var `CLICOLOR_FORCE`
	if _, ok := os.LookupEnv("CLICOLOR_FORCE"); !ok {
		os.Setenv("CLICOLOR_FORCE", "1")
	}

	if opts.Writer == nil {
		opts.Writer = os.Stderr
	}

	level := log.InfoLevel
	if opts.ShowDebugLogs {
		level = log.DebugLevel
	}

	logger := log.NewWithOptions(opts.Writer, log.Options{
		ReportCaller:    opts.ShowCaller,
		ReportTimestamp: opts.ShowTimestamp,
		Prefix:          opts.Prefix,
		Level:           level,
	})

	styles := log.DefaultStyles()
	// styles.Caller = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#5b717f", Light: "#36cbfa"}).Faint(true)
	styles.Caller = lipgloss.NewStyle().Foreground(lipgloss.Color("#878a8a"))

	styles.Levels[log.DebugLevel] = styles.Levels[log.DebugLevel].Foreground(lipgloss.Color("#5b717f"))

	styles.Levels[log.InfoLevel] = styles.Levels[log.InfoLevel].Foreground(lipgloss.Color("#36cbfa"))

	// BUG: due to a bug in termenv, adaptive colors don't work within tmux
	// it always selects the dark variant

	// styles.Levels[log.InfoLevel] = styles.Levels[log.InfoLevel].Foreground(lipgloss.AdaptiveColor{
	// 	Light: string(lipgloss.Color("#36cbfa")),
	// 	Dark:  string(lipgloss.Color("#608798")),
	// })

	styles.Key = lipgloss.NewStyle().Foreground(lipgloss.Color("#36cbfa")).Bold(true)

	logger.SetStyles(styles)

	// output := termenv.NewOutput(os.Stdout, termenv.WithProfile(termenv.TrueColor))
	// logger.Info("theme", "fg", output.ForegroundColor(), "bg", output.BackgroundColor(), "has-dark", output.HasDarkBackground())

	l := slog.New(logger)

	if opts.SetAsDefaultLogger {
		slog.SetDefault(l)
	}

	return l
}
