package log

import (
	"log/slog"
	"os"

	"github.com/fatih/color"
)

var (
	PrintError = color.New(color.FgRed).PrintlnFunc()
	PrintWarn  = color.New(color.FgYellow).PrintlnFunc()
	PrintInfo  = color.New(color.FgCyan).PrintlnFunc()
)

func Info(msg string, v ...interface{}) {
	logLevel := &slog.LevelVar{}
	logLevel.Set(slog.LevelInfo)
	slog.SetDefault(slog.New(New(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})))
	slog.Info(msg, v...)
}

func Debug(msg string, v ...interface{}) {
	if os.Getenv("JERM_VERBOSE") != "1" {
		return
	}
	logLevel := &slog.LevelVar{}
	logLevel.Set(slog.LevelDebug)
	slog.SetDefault(slog.New(New(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})))
	slog.Debug(msg, v...)
}

func Warn(msg string, v ...interface{}) {
	logLevel := &slog.LevelVar{}
	logLevel.Set(slog.LevelWarn)
	slog.SetDefault(slog.New(New(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})))
	slog.Warn(msg, v...)
}

func Error(msg string, v ...interface{}) {
	logLevel := &slog.LevelVar{}
	logLevel.Set(slog.LevelError)
	slog.SetDefault(slog.New(New(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})))
	slog.Error(msg, v...)
}
