// Package config provides shared configuration utilities for the ClawFactory platform.
package config

import (
	"log/slog"
	"strings"
)

// ParseSlogLevel converts a log level string to the corresponding slog.Level.
// Valid values: "debug", "info", "warn", "error". Returns slog.LevelInfo for invalid strings.
func ParseSlogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
