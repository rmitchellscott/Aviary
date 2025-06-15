package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

var jwtSecret []byte
var uiSecret string // UI authentication secret
var (
	loginLimiters sync.Map
	loginRate     = rate.Every(time.Minute / 5) // 5 requests per minute
)

func getLoginLimiter(ip string) *rate.Limiter {
	val, ok := loginLimiters.Load(ip)
	if ok {
		return val.(*rate.Limiter)
	}
	limiter := rate.NewLimiter(loginRate, 5)
	loginLimiters.Store(ip, limiter)
	return limiter
}

func allowInsecure() bool {
	v := strings.ToLower(os.Getenv("ALLOW_INSECURE"))
	return v == "1" || v == "true" || v == "yes"
}

func init() {
	// Generate a random JWT secret if not provided
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		jwtSecret = []byte(secret)
	} else {
		jwtSecret = make([]byte, 32)
		rand.Read(jwtSecret)
	}

	// Generate a random UI secret for internal authentication
	uiSecretBytes := make([]byte, 32)
	rand.Read(uiSecretBytes)
	uiSecret = fmt.Sprintf("%x", uiSecretBytes)
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func LoginHandler(c *gin.Context) {
	// rate limit by client IP
	ip := c.ClientIP()
	if !getLoginLimiter(ip).Allow() {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "backend.auth.too_many_attempts"})
		return
	}

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "backend.auth.invalid_request"})
		return
	}

	// Check credentials against environment variables
	envUsername := os.Getenv("AUTH_USERNAME")
	envPassword := os.Getenv("AUTH_PASSWORD")

	if envUsername == "" || envPassword == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "backend.auth.not_configured"})
		return
	}

	if req.Username != envUsername || req.Password != envPassword {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "backend.auth.invalid_credentials"})
		return
	}

	// Generate JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": req.Username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "backend.auth.token_error"})
		return
	}

	// Set HTTP-only cookie
	secure := !allowInsecure()
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("auth_token", tokenString, 24*3600, "/", "", secure, true)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func LogoutHandler(c *gin.Context) {
	secure := !allowInsecure()
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("auth_token", "", -1, "/", "", secure, true)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// isValidApiKey checks if the request has a valid API key
func isValidApiKey(c *gin.Context) bool {
	envApiKey := os.Getenv("API_KEY")
	if envApiKey == "" {
		return false // No API key configured
	}

	// Check Authorization header (Bearer token)
	if authHeader := c.GetHeader("Authorization"); authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			apiKey := strings.TrimPrefix(authHeader, "Bearer ")
			if subtle.ConstantTimeCompare([]byte(apiKey), []byte(envApiKey)) == 1 {
				return true
			}
		}
	}

	// Check X-API-Key header
	if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
		if subtle.ConstantTimeCompare([]byte(apiKey), []byte(envApiKey)) == 1 {
			return true
		}
	}

	return false
}

// ApiKeyOrJWTMiddleware checks for either valid API key or valid JWT
func ApiKeyOrJWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check API key first
		if isValidApiKey(c) {
			c.Next()
			return
		}

		// Then check JWT cookie
		tokenString, err := c.Cookie("auth_token")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "backend.auth.no_token"})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "backend.auth.invalid_token"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUISecret returns the UI authentication secret for embedding in frontend
func GetUISecret() string {
	return uiSecret
}

func CheckAuthHandler(c *gin.Context) {
	// Check API key first
	if isValidApiKey(c) {
		c.JSON(http.StatusOK, gin.H{"authenticated": true})
		return
	}

	// Check if web authentication is configured
	envUsername := os.Getenv("AUTH_USERNAME")
	envPassword := os.Getenv("AUTH_PASSWORD")
	webAuthEnabled := envUsername != "" && envPassword != ""

	if !webAuthEnabled {
		// Web auth is disabled - check UI secret before auto-generating JWT
		uiToken := c.GetHeader("X-UI-Token")
		if uiToken != uiSecret {
			// No valid UI token - this is likely an external API call
			c.JSON(http.StatusUnauthorized, gin.H{"error": "backend.auth.required"})
			return
		}

		// UI secret is valid - check if they already have a valid JWT
		if tokenString, err := c.Cookie("auth_token"); err == nil {
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return jwtSecret, nil
			})
			if err == nil && token.Valid {
				c.JSON(http.StatusOK, gin.H{"authenticated": true})
				return
			}
		}

		// Generate auto-JWT for UI user
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"username": "ui-user",
			"exp":      time.Now().Add(24 * time.Hour).Unix(),
			"iat":      time.Now().Unix(),
			"auto":     true, // Mark as auto-generated
		})

		tokenString, err := token.SignedString(jwtSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "backend.auth.session_error"})
			return
		}

		// Set HTTP-only cookie
		secure := !allowInsecure()
		c.SetSameSite(http.SameSiteStrictMode)
		c.SetCookie("auth_token", tokenString, 24*3600, "/", "", secure, true)
		c.JSON(http.StatusOK, gin.H{"authenticated": true})
		return
	}

	// Web auth is enabled - check JWT cookie normally
	tokenString, err := c.Cookie("auth_token")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return jwtSecret, nil
	})

	authenticated := err == nil && token.Valid
	c.JSON(http.StatusOK, gin.H{"authenticated": authenticated})
}
