package logging

import (
	"github.com/rs/zerolog"
	"os"
)

type Logger interface {
	Info(msg string)
	Error(err error)
	ErrorStack(err error)
	ErrorWrap(err error, msg string)
	Debug(msg string)
}

type lgr struct {
	logger zerolog.Logger
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

func NewLogger() Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}
	logger := zerolog.New(consoleWriter).With().Timestamp().Logger()

	return &lgr{logger: logger}
}
