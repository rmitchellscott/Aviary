package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/database"
	"github.com/rmitchellscott/aviary/internal/rmapi"
	"github.com/rmitchellscott/aviary/internal/smtp"
	"golang.org/x/crypto/bcrypt"
)

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// PasswordResetRequest represents a password reset request
type PasswordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// PasswordResetConfirmRequest represents a password reset confirmation
type PasswordResetConfirmRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

// UserResponse represents a user in API responses
type UserResponse struct {
	ID                 uuid.UUID  `json:"id"`
	Username           string     `json:"username"`
	Email              string     `json:"email"`
	IsAdmin            bool       `json:"is_admin"`
	IsActive           bool       `json:"is_active"`
	RmapiHost          string     `json:"rmapi_host,omitempty"`
	DefaultRmdir       string     `json:"default_rmdir"`
	CoverpageSetting   string     `json:"coverpage_setting"`
	ConflictResolution string     `json:"conflict_resolution"`
	PageResolution     string     `json:"page_resolution,omitempty"`
	PageDPI            float64    `json:"page_dpi,omitempty"`
	RmapiPaired        bool       `json:"rmapi_paired"`
	CreatedAt          time.Time  `json:"created_at"`
	LastLogin          *time.Time `json:"last_login,omitempty"`
}

// GetRegistrationStatusHandler returns whether registration is enabled (public endpoint)
func GetRegistrationStatusHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}

	// Check if registration is enabled
	regEnabled, err := database.GetSystemSetting("registration_enabled")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"enabled": regEnabled == "true"})
}

// PublicRegisterHandler handles public user registration (when enabled)
func PublicRegisterHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Registration not available in single-user mode"})
		return
	}

	// Check if this would be the first user
	var userCount int64
	if err := database.DB.Model(&database.User{}).Count(&userCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check user count"})
		return
	}

	// Allow registration if no users exist, otherwise check if registration is enabled
	if userCount > 0 {
		regEnabled, err := database.GetSystemSetting("registration_enabled")
		if err != nil || regEnabled != "true" {
			c.JSON(http.StatusForbidden, gin.H{"error": "User registration is disabled"})
			return
		}
	}

	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": validationErrorMessage(err)})
		return
	}

	firstUser := userCount == 0

	userService := database.NewUserService(database.DB)
	newUser, err := userService.CreateUser(req.Username, req.Email, req.Password, firstUser)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, gin.H{"error": "User with this username or email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// If this is the first user, migrate single-user data asynchronously
	if firstUser {
		go func() {
			if err := database.MigrateSingleUserData(newUser.ID); err != nil {
				fmt.Printf("Warning: failed to migrate single-user data: %v\n", err)
			}
		}()
	}

	// Send welcome email if SMTP is configured
	if smtp.IsSMTPConfigured() {
		if err := smtp.SendWelcomeEmail(newUser.Email, newUser.Username); err != nil {
			// Log error but don't fail user creation
			fmt.Printf("Failed to send welcome email: %v\n", err)
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "User created successfully",
	})
}

// RegisterHandler handles user registration (admin only)
func RegisterHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Registration not available in single-user mode"})
		return
	}

	// Check if user is admin
	currentUser, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	user := currentUser.(*database.User)
	if !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin privileges required"})
		return
	}

	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": validationErrorMessage(err)})
		return
	}

	// Admin can always create users regardless of registration_enabled setting

	userService := database.NewUserService(database.DB)
	newUser, err := userService.CreateUser(req.Username, req.Email, req.Password, false)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, gin.H{"error": "User with this username or email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Send welcome email if SMTP is configured
	if smtp.IsSMTPConfigured() {
		if err := smtp.SendWelcomeEmail(newUser.Email, newUser.Username); err != nil {
			// Log error but don't fail user creation
			fmt.Printf("Failed to send welcome email: %v\n", err)
		}
	}

	// Convert to response format
	response := UserResponse{
		ID:                 newUser.ID,
		Username:           newUser.Username,
		Email:              newUser.Email,
		IsAdmin:            newUser.IsAdmin,
		IsActive:           newUser.IsActive,
		RmapiHost:          newUser.RmapiHost,
		RmapiPaired:        rmapi.IsUserPaired(newUser.ID),
		DefaultRmdir:       newUser.DefaultRmdir,
		CoverpageSetting:   newUser.CoverpageSetting,
		ConflictResolution: newUser.ConflictResolution,
		PageResolution:     newUser.PageResolution,
		PageDPI:            newUser.PageDPI,
		CreatedAt:          newUser.CreatedAt,
		LastLogin:          newUser.LastLogin,
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"user":    response,
	})
}

// MultiUserLoginHandler handles login for multi-user mode
func MultiUserLoginHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		// Fallback to single-user login
		LoginHandler(c)
		return
	}

	// Rate limit by client IP
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

	// Log the login attempt
	database.DB.Create(&database.LoginAttempt{
		IPAddress:   ip,
		Username:    req.Username,
		Success:     false,
		AttemptedAt: time.Now(),
		UserAgent:   c.GetHeader("User-Agent"),
	})

        userService := database.NewUserService(database.DB)
        user, err := userService.AuthenticateUser(req.Username, req.Password)
        if err != nil {
                if err.Error() == "account disabled" {
                        c.JSON(http.StatusUnauthorized, gin.H{"error": "backend.auth.account_disabled"})
                } else {
                        c.JSON(http.StatusUnauthorized, gin.H{"error": "backend.auth.invalid_credentials"})
                }
                return
        }

	// Log successful login
	database.DB.Create(&database.LoginAttempt{
		IPAddress:   ip,
		Username:    req.Username,
		Success:     true,
		AttemptedAt: time.Now(),
		UserAgent:   c.GetHeader("User-Agent"),
	})

	// Generate JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  user.ID.String(),
		"username": user.Username,
		"is_admin": user.IsAdmin,
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

	// Also create a session record
	sessionHash, _ := bcrypt.GenerateFromPassword([]byte(tokenString), bcrypt.DefaultCost)
	database.DB.Create(&database.UserSession{
		UserID:    user.ID,
		TokenHash: string(sessionHash),
		ExpiresAt: time.Now().Add(sessionTimeout),
		UserAgent: c.GetHeader("User-Agent"),
		IPAddress: ip,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user": UserResponse{
			ID:                 user.ID,
			Username:           user.Username,
			Email:              user.Email,
			IsAdmin:            user.IsAdmin,
			IsActive:           user.IsActive,
			RmapiHost:          user.RmapiHost,
			RmapiPaired:        rmapi.IsUserPaired(user.ID),
			DefaultRmdir:       user.DefaultRmdir,
			CoverpageSetting:   user.CoverpageSetting,
			ConflictResolution: user.ConflictResolution,
			PageResolution:     user.PageResolution,
			PageDPI:            user.PageDPI,
			CreatedAt:          user.CreatedAt,
			LastLogin:          user.LastLogin,
		},
	})
}

// PasswordResetHandler initiates password reset
func PasswordResetHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Password reset not available in single-user mode"})
		return
	}

	var req PasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	userService := database.NewUserService(database.DB)
	token, err := userService.GeneratePasswordResetToken(req.Email)
	if err != nil {
		// Don't reveal whether email exists or not
		c.JSON(http.StatusOK, gin.H{
			"success": true,
		})
		return
	}

	// Send email with reset token if SMTP is configured
	if smtp.IsSMTPConfigured() {
		// Get user info for email
		user, err := database.GetUserByEmail(req.Email)
		if err == nil {
			if err := smtp.SendPasswordResetEmail(user.Email, user.Username, token); err != nil {
				// Log error but don't reveal it to user
				fmt.Printf("Failed to send password reset email: %v\n", err)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// PasswordResetConfirmHandler confirms password reset
func PasswordResetConfirmHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Password reset not available in single-user mode"})
		return
	}

	var req PasswordResetConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": validationErrorMessage(err)})
		return
	}

	userService := database.NewUserService(database.DB)

	// Validate the reset token
	user, err := userService.ValidatePasswordResetToken(req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset token"})
		return
	}

	// Update the password
	if err := userService.UpdateUserPassword(user.ID, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Password has been reset successfully",
	})
}

// GetCurrentUserHandler returns current user info
func GetCurrentUserHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not available in single-user mode"})
		return
	}

	currentUser, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	user := currentUser.(*database.User)
	response := UserResponse{
		ID:                 user.ID,
		Username:           user.Username,
		Email:              user.Email,
		IsAdmin:            user.IsAdmin,
		IsActive:           user.IsActive,
		RmapiHost:          user.RmapiHost,
		RmapiPaired:        rmapi.IsUserPaired(user.ID),
		DefaultRmdir:       user.DefaultRmdir,
		CoverpageSetting:   user.CoverpageSetting,
		ConflictResolution: user.ConflictResolution,
		PageResolution:     user.PageResolution,
		PageDPI:            user.PageDPI,
		CreatedAt:          user.CreatedAt,
		LastLogin:          user.LastLogin,
	}

	c.JSON(http.StatusOK, response)
}

// MultiUserCheckAuthHandler checks authentication for multi-user mode
func MultiUserCheckAuthHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		// Fallback to single-user check
		CheckAuthHandler(c)
		return
	}

	// Check for valid JWT token
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

	if err != nil || !token.Valid {
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}

	// Extract user ID from token
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}

	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}

	// Verify user still exists and is active
	userService := database.NewUserService(database.DB)
	user, err := userService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"user": UserResponse{
			ID:                 user.ID,
			Username:           user.Username,
			Email:              user.Email,
			IsAdmin:            user.IsAdmin,
			IsActive:           user.IsActive,
			RmapiHost:          user.RmapiHost,
			RmapiPaired:        rmapi.IsUserPaired(user.ID),
			DefaultRmdir:       user.DefaultRmdir,
			CoverpageSetting:   user.CoverpageSetting,
			ConflictResolution: user.ConflictResolution,
			PageResolution:     user.PageResolution,
			PageDPI:            user.PageDPI,
			CreatedAt:          user.CreatedAt,
			LastLogin:          user.LastLogin,
		},
	})
}

// extractUserFromToken extracts user information from JWT token
func extractUserFromToken(tokenString string) (*database.User, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		return nil, errors.New("invalid user ID in token")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, errors.New("invalid user ID format")
	}

	userService := database.NewUserService(database.DB)
	return userService.GetUserByID(userID)
}

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", bytes), nil
}
