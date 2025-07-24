package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/database"
)

var (
	proxyAuthEnabled bool
	proxyHeaderName  string
)

// InitProxyAuth initializes proxy authentication configuration
func InitProxyAuth() {
	headerName := config.Get("PROXY_AUTH_HEADER", "")
	if headerName == "" {
		proxyAuthEnabled = false
		return
	}

	proxyAuthEnabled = true
	proxyHeaderName = headerName
}

// getProxyHeaderName returns the configured proxy header name
func getProxyHeaderName() string {
	return proxyHeaderName
}

// IsProxyAuthEnabled returns true if proxy authentication is configured
func IsProxyAuthEnabled() bool {
	return proxyAuthEnabled
}

// ProxyAuthMiddleware handles proxy authentication
func ProxyAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !proxyAuthEnabled {
			c.Next()
			return
		}

		// Get username from proxy header
		username := c.GetHeader(proxyHeaderName)
		if username == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Proxy authentication header missing"})
			c.Abort()
			return
		}

		// Clean username
		username = strings.TrimSpace(username)
		if username == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Empty username in proxy header"})
			c.Abort()
			return
		}

		// Handle authentication based on mode
		if database.IsMultiUserMode() {
			// Multi-user mode: check if user exists in database
			user, err := database.GetUserByUsername(username)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in database"})
				c.Abort()
				return
			}

			// Check if user is active
			if !user.IsActive {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "User account is deactivated"})
				c.Abort()
				return
			}

			c.Set("user", user)
			c.Set("auth_method", "proxy")
		} else {
			// Single-user mode - create a dummy user with username from header
			user := &database.User{
				Username: username,
				IsAdmin:  true, // Single-user mode user is always admin
			}
			c.Set("user", user)
			c.Set("auth_method", "proxy")
		}

		c.Next()
	}
}

// ProxyAuthCheckHandler checks proxy authentication status
func ProxyAuthCheckHandler(c *gin.Context) {
	if !proxyAuthEnabled {
		c.JSON(http.StatusOK, gin.H{
			"authenticated": false,
			"proxy_auth":    false,
		})
		return
	}

	username := c.GetHeader(proxyHeaderName)
	if username == "" {
		// Proxy auth is enabled but header is missing - not an error, just not authenticated via proxy
		c.JSON(http.StatusOK, gin.H{
			"authenticated":   false,
			"proxy_auth":      true,
			"proxy_available": false,
			"message":         "Proxy authentication header not present",
		})
		return
	}

	username = strings.TrimSpace(username)
	if username == "" {
		c.JSON(http.StatusOK, gin.H{
			"authenticated": false,
			"proxy_auth":    true,
			"error":         "Empty username in proxy header",
		})
		return
	}

	// In multi-user mode, check if user exists and is active
	if database.IsMultiUserMode() {
		user, err := database.GetUserByUsername(username)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"authenticated": false,
				"proxy_auth":    true,
				"error":         "User not found in database",
			})
			return
		}

		if !user.IsActive {
			c.JSON(http.StatusOK, gin.H{
				"authenticated": false,
				"proxy_auth":    true,
				"error":         "User account is deactivated",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"authenticated": true,
			"proxy_auth":    true,
			"user": gin.H{
				"id":       user.ID,
				"username": user.Username,
				"email":    user.Email,
				"is_admin": user.IsAdmin,
			},
		})
	} else {
		// Single-user mode - always successful if header is present
		c.JSON(http.StatusOK, gin.H{
			"authenticated": true,
			"proxy_auth":    true,
			"username":      username,
		})
	}
}
