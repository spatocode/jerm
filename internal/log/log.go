package log

import (
	"log/slog"

	"github.com/fatih/color"
)

var (
	PrintError = color.New(color.FgRed).PrintlnFunc()
	PrintWarn  = color.New(color.FgYellow).PrintlnFunc()
	PrintInfo  = color.New(color.FgCyan).PrintlnFunc()
)

func Info(msg string, v ...interface{}) {
	slog.Info(msg, v...)
}

func Debug(msg string, v ...interface{}) {
	slog.Info(msg, v...)
}

func Warn(msg string, v ...interface{}) {
	slog.Warn(msg, v...)
}

func Error(msg string, v ...interface{}) {
	slog.Error(msg, v...)
}
