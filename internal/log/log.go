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
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	logger.Info(msg, v...)
}

func Debug(msg string, v ...interface{}) {
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	logger.Debug(msg, v...)
}

func Warn(msg string, v ...interface{}) {
	opts := &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	logger.Warn(msg, v...)
}

func Error(msg string, v ...interface{}) {
	opts := &slog.HandlerOptions{
		Level: slog.LevelError,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	logger.Error(msg, v...)
}
