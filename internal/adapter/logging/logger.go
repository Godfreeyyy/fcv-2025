package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ZapLogger implements the Logger interface with zap
type ZapLogger struct {
	logger *zap.SugaredLogger
}

// NewZapLogger creates a new zap logger
func NewZapLogger() *ZapLogger {
	// Create a basic logger configuration
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "time"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Create the logger
	logger, _ := config.Build()
	sugar := logger.Sugar()

	return &ZapLogger{
		logger: sugar,
	}
}

// Info logs an info message
func (l *ZapLogger) Info(msg string, args ...interface{}) {
	l.logger.Infow(msg, args...)
}

// Error logs an error message
func (l *ZapLogger) Error(msg string, args ...interface{}) {
	l.logger.Errorw(msg, args...)
}

// Debug logs a debug message
func (l *ZapLogger) Debug(msg string, args ...interface{}) {
	l.logger.Debugw(msg, args...)
}

// Warn logs a warning message
func (l *ZapLogger) Warn(msg string, args ...interface{}) {
	l.logger.Warnw(msg, args...)
}
