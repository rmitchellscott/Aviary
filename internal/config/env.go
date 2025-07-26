package config

import (
	"os"
	"strconv"
	"strings"
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
