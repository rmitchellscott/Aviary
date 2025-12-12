package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Get returns the value of the environment variable `key` if set.
// If not set, and `key + "_FILE"` is set, the file at that path is read and
// its trimmed contents are returned. If neither are set, def is returned.
func Get(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	if path := os.Getenv(key + "_FILE"); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return def
}

// GetInt returns the integer value of the environment variable `key`.
// It parses the result of Get(key, ""). If parsing fails or the variable is
// unset, def is returned.
func GetInt(key string, def int) int {
	if val := Get(key, ""); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return def
}

// GetBool returns the boolean value of the environment variable `key`.
// Recognised true values are: 1, t, true, y, yes (case-insensitive).
// Recognised false values are: 0, f, false, n, no.
func GetBool(key string, def bool) bool {
	if val := Get(key, ""); val != "" {
		switch strings.ToLower(val) {
		case "1", "t", "true", "y", "yes":
			return true
		case "0", "f", "false", "n", "no":
			return false
		}
	}
	return def
}

// ParseDuration parses a duration string. It behaves like time.ParseDuration
// but also supports values like "30d" to represent days.
func ParseDuration(s string) (time.Duration, error) {
	lower := strings.ToLower(strings.TrimSpace(s))
	if strings.HasSuffix(lower, "d") {
		days := strings.TrimSuffix(lower, "d")
		if n, err := strconv.Atoi(days); err == nil {
			return time.Duration(n) * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(lower)
}

// GetDuration returns the duration value of the environment variable `key`.
// It uses ParseDuration and falls back to def if parsing fails or the variable
// is unset.
func GetDuration(key string, def time.Duration) time.Duration {
	if val := Get(key, ""); val != "" {
		if d, err := ParseDuration(val); err == nil {
			return d
		}
	}
	return def
}

// GetConversionOutputFormat returns the preferred output format (pdf or epub).
// It checks for CONVERSION_OUTPUT_FORMAT environment variable and validates the value.
// Valid values are "pdf" and "epub" (case-insensitive).
// Defaults to "epub" if unset or invalid.
func GetConversionOutputFormat() string {
	format := strings.ToLower(Get("CONVERSION_OUTPUT_FORMAT", "epub"))
	if format == "epub" || format == "pdf" {
		return format
	}
	return "epub"
}
