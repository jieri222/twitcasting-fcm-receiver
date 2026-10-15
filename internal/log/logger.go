// Package log configures the application's structured console logger.
package log

import (
	"log/slog"
	"os"
)

var level = new(slog.LevelVar)

// InitLogger configures JSON logs suitable for terminals and container log collectors.
func InitLogger(verbose bool) {
	level.Set(slog.LevelInfo)
	if verbose {
		level.Set(slog.LevelDebug)
	}
	// Keep logs on stderr so stdout remains usable for notification payloads and CLI output.
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler).With("service", "twitcasting-fcm-receiver"))
}

// SetLogLevel changes the minimum emitted log level.
func SetLogLevel(next slog.Level) {
	level.Set(next)
}

func Debug(message string, args ...any) { slog.Debug(message, args...) }
func Info(message string, args ...any)  { slog.Info(message, args...) }
func Warn(message string, args ...any)  { slog.Warn(message, args...) }
func Error(message string, args ...any) { slog.Error(message, args...) }
