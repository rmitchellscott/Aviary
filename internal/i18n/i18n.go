package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// findLocaleFile tries to locate the locale file in various possible locations
func findLocaleFile(lang string) string {
	filename := fmt.Sprintf("%s.json", lang)
	
	// Try various possible locations
	possiblePaths := []string{
		filepath.Join("locales", filename),           // ./locales/en.json (working dir)
		filepath.Join("/app", "locales", filename),   // /app/locales/en.json (Docker)
		filepath.Join(".", "locales", filename),      // ./locales/en.json (explicit relative)
	}
	
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	
	// Default to the first option if none found
	return possiblePaths[0]
}

// Localizer handles translation lookups
type Localizer struct {
	lang string
	data map[string]interface{}
}

// New creates a new localizer for the given language
func New(lang string) (*Localizer, error) {
	// Try to find the locale file for the requested language
	filename := findLocaleFile(lang)
	content, err := os.ReadFile(filename)
	if err != nil {
		// If the requested language file doesn't exist, fall back to English
		if lang != "en" {
			return New("en")
		}
		return nil, fmt.Errorf("failed to read locale file %s: %w", filename, err)
	}

	// Parse JSON
	var data map[string]interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("failed to parse locale file %s: %w", filename, err)
	}

	return &Localizer{
		lang: lang,
		data: data,
	}, nil
}

// T translates a key path like "backend.auth.invalid_credentials"
func (l *Localizer) T(key string) string {
	return l.TWithData(key, nil)
}

// TWithData translates a key with template data for interpolation
func (l *Localizer) TWithData(key string, data map[string]string) string {
	parts := strings.Split(key, ".")
	current := l.data

	// Navigate through the nested structure
	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - should be a string
			if val, ok := current[part].(string); ok {
				return l.interpolate(val, data)
			}
		} else {
			// Intermediate part - should be a map
			if next, ok := current[part].(map[string]interface{}); ok {
				current = next
			} else {
				break // Path not found
			}
		}
	}

	// Return the key if not found (for debugging)
	return key
}

// interpolate replaces {{key}} placeholders with values from data
func (l *Localizer) interpolate(text string, data map[string]string) string {
	if data == nil {
		return text
	}

	result := text
	for key, value := range data {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// Lang returns the current language
func (l *Localizer) Lang() string {
	return l.lang
}

// Global localizer for convenience
var defaultLocalizer *Localizer

// init initializes the default localizer with English
func init() {
	var err error
	defaultLocalizer, err = New("en")
	if err != nil {
		// During tests or when locale files aren't available, create a minimal fallback localizer
		defaultLocalizer = &Localizer{
			lang: "en",
			data: map[string]interface{}{
				"backend": map[string]interface{}{
					"status": map[string]interface{}{
						"done": "Done",
						"uploading": "Uploading",
						"downloading": "Downloading",
						"converting_pdf": "Converting to PDF",
						"compressing_pdf": "Compressing PDF",
						"upload_success": "Your document is available on your reMarkable at {{path}}",
					},
					"errors": map[string]interface{}{
						"internal_error": "Internal error",
					},
				},
			},
		}
	}
}

// SetLanguage sets the global language
func SetLanguage(lang string) error {
	var err error
	defaultLocalizer, err = New(lang)
	return err
}

// T is a convenience function for the global localizer
func T(key string) string {
	return defaultLocalizer.T(key)
}

// TWithData is a convenience function for the global localizer
func TWithData(key string, data map[string]string) string {
	return defaultLocalizer.TWithData(key, data)
}