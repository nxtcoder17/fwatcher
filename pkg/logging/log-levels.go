package logging

import "log/slog"

// Define a custom Trace level
const (
	DebugVerbose1Level = slog.Level(-4)
	DebugVerbose2Level = slog.Level(-8)
	DebugVerbose3Level = slog.Level(-12)
)
