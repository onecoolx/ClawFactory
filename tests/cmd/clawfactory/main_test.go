package clawfactory_test

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/clawfactory/clawfactory/internal/config"
	"pgregory.net/rapid"
)

// Feature: v03-observability, Property 36: 日志级别解析正确性
// For any valid log level string ("debug", "info", "warn", "error"), ParseSlogLevel returns
// the corresponding slog.Level; for invalid strings, returns default slog.LevelInfo.
// **Validates: Requirements 2.2**
func TestProperty36_LogLevelParseCorrectness(t *testing.T) {
	validLevels := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}

	rapid.Check(t, func(t *rapid.T) {
		// Draw from a mix of valid and invalid level strings
		level := rapid.SampledFrom([]string{
			"debug", "info", "warn", "error",
			"DEBUG", "INFO", "WARN", "ERROR",
			"Debug", "Info", "Warn", "Error",
			"", "trace", "fatal", "unknown", "verbose", "off",
			"warning", "err", "dbg", "information",
		}).Draw(t, "level")

		got := config.ParseSlogLevel(level)

		// Check if it's a valid level (case-insensitive)
		expected, isValid := validLevels[strings.ToLower(level)]
		if isValid {
			if got != expected {
				t.Fatalf("ParseSlogLevel(%q) = %v, want %v", level, got, expected)
			}
		} else {
			// Invalid strings should return default slog.LevelInfo
			if got != slog.LevelInfo {
				t.Fatalf("ParseSlogLevel(%q) = %v, want default slog.LevelInfo (%v)", level, got, slog.LevelInfo)
			}
		}
	})
}
