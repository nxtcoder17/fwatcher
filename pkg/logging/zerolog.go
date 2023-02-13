package logging

import (
	"github.com/rs/zerolog"
	"os"
)

type lgr struct {
	logger zerolog.Logger
	lvlMap map[LogLevel]zerolog.Level
}

func (l lgr) SetLogLevel(lvl LogLevel) {
	zerolog.SetGlobalLevel(l.lvlMap[lvl])
}

func (l lgr) ErrorWrap(err error, msg string) {
	l.logger.Err(err).Msg(msg)
}

func (l lgr) Debug(msg string) {
	l.logger.Debug().Msg(msg)
}

func (l lgr) ErrorStack(err error) {
	l.logger.Error().Stack().Err(err).Msg("")
}

func (l lgr) Error(err error) {
	l.logger.Err(err).Msg("")
}

func (l lgr) Info(msg string) {
	l.logger.Info().Msgf(msg)
}

func newZeroLogger() Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}
	logger := zerolog.New(consoleWriter).With().Timestamp().Logger()

	lvlMap := map[LogLevel]zerolog.Level{
		TraceLevel: zerolog.TraceLevel,
		DebugLevel: zerolog.DebugLevel,
		InfoLevel:  zerolog.InfoLevel,
		WarnLevel:  zerolog.WarnLevel,
		ErrorLevel: zerolog.ErrorLevel,
		PanicLevel: zerolog.PanicLevel,
	}

	return &lgr{logger: logger, lvlMap: lvlMap}
}
