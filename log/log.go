package log

import "context"

// Logger defines interface of structured logger
type Logger interface {
	With(logger Logger, keyValues ...interface{}) Logger
	Info(message string, keyValues ...interface{})
	Error(message string, keyValues ...interface{})
}

// New is the constructor of logger that triggers when it absents in context.
// you should redefine it with you logger constructor.
var New = func() Logger {
	return dummyLogger{}
}

type logKeyType struct{}

var logKey = logKeyType{}

func NewContext(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, logKey, logger)
}

func FromContext(ctx context.Context) Logger {
	logger, _ := ctx.Value(logKey).(Logger)
	if logger == nil {
		logger = New()
	}

	return logger
}

// default logger that does nothing
type dummyLogger struct{}

func (dummyLogger) With(logger Logger, keyValues ...interface{}) Logger { return dummyLogger{} }
func (dummyLogger) Info(message string, keyValues ...interface{})       {}
func (dummyLogger) Error(message string, keyValues ...interface{})      {}
