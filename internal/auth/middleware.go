package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/database"
)

// MultiUserAuthMiddleware provides authentication for multi-user mode
func MultiUserAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !database.IsMultiUserMode() {
			// Single-user mode - use traditional auth only
			ApiKeyOrJWTMiddleware()(c)
			return
		}

		// Check proxy auth first if header is present
		if IsProxyAuthEnabled() {
			username := c.GetHeader(getProxyHeaderName())
			if username != "" {
				// Proxy header present, use proxy auth
				ProxyAuthMiddleware()(c)
				return
			}
			// Proxy auth is enabled but header not present, fall through to other auth methods
		}

		// Check API key next (for programmatic access)
		if user := checkAPIKey(c); user != nil {
			c.Set("user", user)
			c.Set("auth_method", "api_key")
			c.Next()
			return
		}

		// Check JWT token
		if user := checkJWTToken(c); user != nil {
			c.Set("user", user)
			c.Set("auth_method", "jwt")
			c.Next()
			return
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		c.Abort()
	}
}

// AdminRequiredMiddleware ensures the user is an admin
func AdminRequiredMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !database.IsMultiUserMode() {
			// In single-user mode, everyone is admin
			c.Next()
			return
		}

		currentUser, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		user := currentUser.(*database.User)
		if !user.IsAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin privileges required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// OptionalAuthMiddleware provides optional authentication
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !database.IsMultiUserMode() {
			// In single-user mode, skip auth if not configured
			envApiKey := config.Get("API_KEY", "")
			envUsername := config.Get("AUTH_USERNAME", "")
			envPassword := config.Get("AUTH_PASSWORD", "")
			authConfigured := envApiKey != "" || (envUsername != "" && envPassword != "")

			if !authConfigured {
				c.Next()
				return
			}
			// Otherwise use the existing middleware
			ApiKeyOrJWTMiddleware()(c)
			return
		}

		// Check API key first
		if user := checkAPIKey(c); user != nil {
			c.Set("user", user)
			c.Set("auth_method", "api_key")
			c.Next()
			return
		}

		// Check JWT token
		if user := checkJWTToken(c); user != nil {
			c.Set("user", user)
			c.Set("auth_method", "jwt")
			c.Next()
			return
		}

		// No authentication found - this is okay for optional auth
		c.Next()
	}
}

// checkAPIKey validates API key and returns user if valid
func checkAPIKey(c *gin.Context) *database.User {
	if !database.IsMultiUserMode() {
		// Single-user mode API key check
		if isValidApiKey(c) {
			// Return a dummy user for single-user mode
			return &database.User{
				Username: "single-user",
				IsAdmin:  true,
			}
		}
		return nil
	}

	// Multi-user mode API key check
	var apiKey string

	// Check Authorization header (Bearer token)
	if authHeader := c.GetHeader("Authorization"); authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			apiKey = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	// Check X-API-Key header
	if apiKey == "" {
		apiKey = c.GetHeader("X-API-Key")
	}

	if apiKey == "" {
		return nil
	}

	// Validate API key against database
	apiKeyService := database.NewAPIKeyService(database.DB)
	user, err := apiKeyService.ValidateAPIKeyConstantTime(apiKey)
	if err != nil {
		return nil
	}

	return user
}

// checkJWTToken validates JWT token and returns user if valid
func checkJWTToken(c *gin.Context) *database.User {
	tokenString, err := c.Cookie("auth_token")
	if err != nil {
		return nil
	}

	if !database.IsMultiUserMode() {
		// Single-user mode JWT check
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			return nil
		}

		// Return a dummy user for single-user mode
		return &database.User{
			Username: "single-user",
			IsAdmin:  true,
		}
	}

	// Multi-user mode JWT check
	user, err := extractUserFromToken(tokenString)
	if err != nil {
		return nil
	}

	return user
}

// GetCurrentUser returns the current authenticated user
func GetCurrentUser(c *gin.Context) *database.User {
	if user, exists := c.Get("user"); exists {
		return user.(*database.User)
	}
	return nil
}

// GetCurrentUserID returns the current authenticated user's ID
func GetCurrentUserID(c *gin.Context) uuid.UUID {
	if user := GetCurrentUser(c); user != nil {
		return user.ID
	}
	return uuid.Nil
}

// IsCurrentUserAdmin checks if the current user is an admin
func IsCurrentUserAdmin(c *gin.Context) bool {
	if user := GetCurrentUser(c); user != nil {
		return user.IsAdmin
	}
	return false
}

// GetAuthMethod returns the authentication method used
func GetAuthMethod(c *gin.Context) string {
	if method, exists := c.Get("auth_method"); exists {
		return method.(string)
	}
	return ""
}

// RequireUser ensures a user is authenticated and returns it
func RequireUser(c *gin.Context) (*database.User, bool) {
	user := GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return nil, false
	}
	return user, true
}

// RequireAdmin ensures the user is an admin and returns it
func RequireAdmin(c *gin.Context) (*database.User, bool) {
	user, ok := RequireUser(c)
	if !ok {
		return nil, false
	}

	if !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin privileges required"})
		return nil, false
	}

	return user, true
}
