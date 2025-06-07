package auth

import (
	"crypto/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret []byte

func init() {
	// Generate a random JWT secret if not provided
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		jwtSecret = []byte(secret)
	} else {
		jwtSecret = make([]byte, 32)
		rand.Read(jwtSecret)
	}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func LoginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Check credentials against environment variables
	envUsername := os.Getenv("AUTH_USERNAME")
	envPassword := os.Getenv("AUTH_PASSWORD")

	if envUsername == "" || envPassword == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication not configured"})
		return
	}

	if req.Username != envUsername || req.Password != envPassword {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
		return
	}

	// Set HTTP-only cookie
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("auth_token", tokenString, 24*3600, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func LogoutHandler(c *gin.Context) {
	c.SetCookie("auth_token", "", -1, "/", "", false, true)
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
			return apiKey == envApiKey
		}
	}

	// Check X-API-Key header
	if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
		return apiKey == envApiKey
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
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No auth token or API key"})
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
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func CheckAuthHandler(c *gin.Context) {
	// Check API key first
	if isValidApiKey(c) {
		c.JSON(http.StatusOK, gin.H{"authenticated": true})
		return
	}

	// Then check JWT cookie
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
