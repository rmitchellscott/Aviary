package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/database"
	"golang.org/x/term"
	"golang.org/x/time/rate"
)

var jwtSecret []byte
var uiSecret string // UI authentication secret
var (
	loginLimiters sync.Map
	loginRate     = rate.Every(time.Minute / 5) // 5 requests per minute
)

// Default session timeout is 24 hours, can be overridden via SESSION_TIMEOUT env var.
var sessionTimeout = 24 * time.Hour

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
	v := strings.ToLower(config.Get("ALLOW_INSECURE", ""))
	return v == "1" || v == "true" || v == "yes"
}

func init() {
	// Generate a random JWT secret if not provided
	if secret := config.Get("JWT_SECRET", ""); secret != "" {
		jwtSecret = []byte(secret)
	} else {
		jwtSecret = make([]byte, 32)
		rand.Read(jwtSecret)
	}

	// Generate a random UI secret for internal authentication
	uiSecretBytes := make([]byte, 32)
	rand.Read(uiSecretBytes)
	uiSecret = fmt.Sprintf("%x", uiSecretBytes)

	// Read session timeout from environment
	sessionTimeout = config.GetDuration("SESSION_TIMEOUT", 24*time.Hour)
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
	envUsername := config.Get("AUTH_USERNAME", "")
	envPassword := config.Get("AUTH_PASSWORD", "")

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
		"exp":      time.Now().Add(sessionTimeout).Unix(),
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
	c.SetCookie("auth_token", tokenString, int(sessionTimeout.Seconds()), "/", "", secure, true)

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
	envApiKey := config.Get("API_KEY", "")
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
	envUsername := config.Get("AUTH_USERNAME", "")
	envPassword := config.Get("AUTH_PASSWORD", "")
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
			"exp":      time.Now().Add(sessionTimeout).Unix(),
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
		c.SetCookie("auth_token", tokenString, int(sessionTimeout.Seconds()), "/", "", secure, true)
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

// AuthRequired checks if API authentication is configured
func AuthRequired() bool {
	envApiKey := config.Get("API_KEY", "")
	return envApiKey != ""
}

// CheckSingleUserPaired checks if rmapi.conf exists for single-user mode
func CheckSingleUserPaired() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	cfgPath := filepath.Join(home, ".config", "rmapi", "rmapi.conf")
	info, err := os.Stat(cfgPath)
	return err == nil && info.Size() > 0
}

// ServeIndexWithSecret serves index.html with injected secret
func ServeIndexWithSecret(c *gin.Context, uiFS fs.FS, secret string) {
	content, err := fs.ReadFile(uiFS, "index.html")
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	// Inject secret into HTML
	html := string(content)
	// Look for </head> tag and inject script before it
	scriptTag := fmt.Sprintf(`<script>window.__UI_SECRET__ = "%s";</script></head>`, secret)
	html = strings.Replace(html, "</head>", scriptTag, 1)

	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, html)
}

// RunPair handles interactive pairing flow
func RunPair(stdout, stderr io.Writer) error {
	// 1) Are we interactive?
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("no TTY detected; please run `docker run ... aviary pair` in an interactive shell")
	}

	if host := config.Get("RMAPI_HOST", ""); host != "" {
		fmt.Fprintf(stdout, "Welcome to Aviary. Let's pair with %s!\n", host)
	} else {
		fmt.Fprintln(stdout, "Welcome to Aviary. Let's pair with the reMarkable Cloud!")
	}

	// 2) cd into rmapi
	cmd := exec.Command("rmapi", "cd")
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("`rmapi cd` failed: %w", err)
	}

	// 3) print the rmapi.conf if it exists
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not get home directory: %w", err)
	}
	cfgPath := filepath.Join(home, ".config", "rmapi", "rmapi.conf")

	fmt.Fprintf(stdout, "\nPrinting your %s file:\n", cfgPath)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("could not read config: %w", err)
	}
	stdout.Write(data)
	stdout.Write([]byte("\n"))

	fmt.Fprintln(stdout, "Done!")
	return nil
}

// HandlePairRequest handles pairing for both single-user and multi-user modes
func HandlePairRequest(c *gin.Context) {
	if database.IsMultiUserMode() {
		// In multi-user mode, delegate to the existing handler
		PairRMAPIHandler(c)
		return
	}

	// Single-user mode pairing
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Ensure the rmapi config directory exists
	home, err := os.UserHomeDir()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to determine home directory"})
		return
	}

	cfgDir := filepath.Join(home, ".config", "rmapi")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create config directory"})
		return
	}

	cfgPath := filepath.Join(cfgDir, "rmapi.conf")

	// Run rmapi pairing command
	cmd := exec.Command("rmapi", "cd")
	cmd.Stdin = strings.NewReader(req.Code + "\n")
	env := os.Environ()
	env = append(env, "RMAPI_CONFIG="+cfgPath)
	if host := config.Get("RMAPI_HOST", ""); host != "" {
		env = append(env, "RMAPI_HOST="+host)
	}
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Pairing failed"})
		return
	}

	// Call post-pairing callback if set (async for folder cache refresh)
	if GetPostPairingCallback() != nil {
		go GetPostPairingCallback()("single-user", true) // true = single-user mode
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
