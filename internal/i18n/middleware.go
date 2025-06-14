package i18n

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// langKey is the key used to store language in context
type langKey struct{}

// LanguageMiddleware detects the user's language preference and sets it in context
func LanguageMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := detectLanguage(c)
		
		// Set language in context for this request
		ctx := context.WithValue(c.Request.Context(), langKey{}, lang)
		c.Request = c.Request.WithContext(ctx)
		
		c.Next()
	}
}

// detectLanguage determines the user's preferred language from various sources
func detectLanguage(c *gin.Context) string {
	// 1. Check query parameter (?lang=es)
	if lang := c.Query("lang"); lang != "" {
		if isValidLang(lang) {
			fmt.Printf("[i18n] Language from query param: %s\n", lang)
			return lang
		}
	}
	
	// 2. Check header (Accept-Language)
	if acceptLang := c.GetHeader("Accept-Language"); acceptLang != "" {
		fmt.Printf("[i18n] Accept-Language header: %s\n", acceptLang)
		// Parse Accept-Language header (e.g., "en-US,en;q=0.9,es;q=0.8")
		languages := strings.Split(acceptLang, ",")
		for _, l := range languages {
			// Extract just the language part (before any semicolon)
			lang := strings.TrimSpace(strings.Split(l, ";")[0])
			
			// Handle language-region format (e.g., "en-US" -> "en")
			if parts := strings.Split(lang, "-"); len(parts) > 0 {
				lang = parts[0]
			}
			
			if isValidLang(lang) {
				fmt.Printf("[i18n] Detected language: %s\n", lang)
				return lang
			}
		}
	}
	
	// 3. Default to English
	fmt.Printf("[i18n] Defaulting to English\n")
	return "en"
}

// isValidLang checks if the language is supported by attempting to create a localizer
func isValidLang(lang string) bool {
	// Try to create a localizer for this language
	// The New() function will automatically fall back to English if the language file doesn't exist
	_, err := New(lang)
	return err == nil
}

// GetLanguageFromContext extracts the language from the request context
func GetLanguageFromContext(ctx context.Context) string {
	if lang, ok := ctx.Value(langKey{}).(string); ok {
		return lang
	}
	return "en" // default
}

// TFromContext translates a key using the language from context
func TFromContext(ctx context.Context, key string) string {
	lang := GetLanguageFromContext(ctx)
	localizer, err := New(lang)
	if err != nil {
		// Fallback to default if error
		return T(key)
	}
	return localizer.T(key)
}

// TWithDataFromContext translates a key with data using the language from context
func TWithDataFromContext(ctx context.Context, key string, data map[string]string) string {
	lang := GetLanguageFromContext(ctx)
	localizer, err := New(lang)
	if err != nil {
		// Fallback to default if error
		return TWithData(key, data)
	}
	return localizer.TWithData(key, data)
}