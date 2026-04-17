package telemetry

import (
	"log/slog"
	"os"
)

type Logger struct {
	base *slog.Logger
}

func New(level string) *Logger {
	var slogLevel slog.Level

	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slogLevel,
	})

	return &Logger{
		base: slog.New(handler),
	}
}

func (l *Logger) Slog() *slog.Logger {
	return l.base
}

func (l *Logger) Info(msg string, args ...any) {
	l.base.Info(msg, args...)
}
