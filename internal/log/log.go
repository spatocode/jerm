package log

import (
	"log/slog"

	"github.com/fatih/color"
)

var (
	PrintError  = color.New(color.FgRed).PrintlnFunc()
	PrintfError = color.New(color.FgRed).PrintfFunc()
	PrintWarn   = color.New(color.FgYellow).PrintlnFunc()
	PrintfWarn  = color.New(color.FgYellow).PrintfFunc()
	PrintInfo   = color.New(color.FgCyan).PrintlnFunc()
	PrintfInfo  = color.New(color.FgCyan).PrintfFunc()
	Yellow      = color.New(color.FgYellow).SprintFunc()
	Red         = color.New(color.FgRed).SprintFunc()
	Blue        = color.New(color.FgBlue).SprintFunc()
	Green       = color.New(color.FgGreen).SprintFunc()
	White       = color.New(color.FgWhite).SprintFunc()
	Magenta     = color.New(color.FgMagenta).SprintFunc()
)

func Info(msg string, v ...interface{}) {
	slog.Info(msg, v...)
}

func Debug(msg string, v ...interface{}) {
	slog.Debug(msg, v...)
}

func Warn(msg string, v ...interface{}) {
	slog.Warn(msg, v...)
}

func Error(msg string, v ...interface{}) {
	slog.Error(msg, v...)
}
