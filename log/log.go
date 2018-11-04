package log

import "context"

// Logger defines interface of structured logger
type Logger interface {
	Info(message string, keyValues ...interface{})
	Error(message string, keyValues ...interface{})
}

// LoggerWith defines interface of logger that could keep added fields
type LoggerWith interface {
	With(keyValues ...interface{}) Logger
}

// Error is shorthand for FromContext(ctx).Error(message, keyValues...)
func Error(ctx context.Context, message string, keyValues ...interface{}) {
	FromContext(ctx).Error(message, keyValues...)
}

// Info is shorthand for FromContext(ctx).Info(message, keyValues...)
func Info(ctx context.Context, message string, keyValues ...interface{}) {
	FromContext(ctx).Info(message, keyValues...)
}

// With is a helper function, it checks if Logger support With(), adds new keyValues and preserves new logger in context
// if logger dones't support With() , it does nothing and returns the same context
func With(ctx context.Context, keyValues ...interface{}) context.Context {
	lw, ok := FromContext(ctx).(LoggerWith)
	if !ok {
		return ctx
	}

	logger := lw.With(keyValues)
	ctx = NewContext(ctx, logger)
	return ctx
}

// New is the constructor of logger that triggers when it absents in context.
// you should redefine it with you logger constructor.
var New = func() Logger {
	return dummyLogger{}
}

// default logger that does nothing
type dummyLogger struct{}

func (dummyLogger) Info(message string, keyValues ...interface{})  {}
func (dummyLogger) Error(message string, keyValues ...interface{}) {}

/*************************** context **********************/
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
