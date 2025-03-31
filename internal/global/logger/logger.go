package logger

import "gitlab.com/fcv-2025.net/internal/adapter/logging"

var Logger = logging.NewZapLogger()

func Info(msg string, args ...interface{}) {
	Logger.Info(msg, args)
}

func Error(msg string, args ...interface{}) {
	Logger.Error(msg, args)
}

func Debug(msg string, args ...interface{}) {
	Logger.Debug(msg, args)
}

func Warn(msg string, args ...interface{}) {
	Logger.Warn(msg, args)
}
