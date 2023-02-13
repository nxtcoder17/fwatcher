package logging

type LogLevel string

const (
	TraceLevel = "TRACE"
	DebugLevel = "DEBUG"
	InfoLevel  = "INFO"
	WarnLevel  = "WARN"
	ErrorLevel = "ERROR"
	PanicLevel = "PANIC"
)

type Logger interface {
	SetLogLevel(lvl LogLevel)
	Info(msg string)
	Error(err error)
	ErrorStack(err error)
	ErrorWrap(err error, msg string)
	Debug(msg string)
}

func NewLogger() Logger {
	return newZeroLogger()
}
