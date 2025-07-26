package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/database"
	"golang.org/x/oauth2"
)

var (
	oidcProvider *oidc.Provider
	oauth2Config *oauth2.Config
	oidcVerifier *oidc.IDTokenVerifier
	oidcEnabled  bool
)

type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// InitOIDC initializes OIDC configuration from environment variables
func InitOIDC() error {
	issuer := config.Get("OIDC_ISSUER", "")
	clientID := config.Get("OIDC_CLIENT_ID", "")
	clientSecret := config.Get("OIDC_CLIENT_SECRET", "")
	redirectURL := config.Get("OIDC_REDIRECT_URL", "")

	if issuer == "" || clientID == "" || clientSecret == "" {
		oidcEnabled = false
		return nil // OIDC not configured, which is fine
	}

	if redirectURL == "" {
		redirectURL = "/api/auth/oidc/callback"
	}

	// Parse scopes from environment
	scopesEnv := config.Get("OIDC_SCOPES", "")
	scopes := []string{"openid", "profile", "email"}
	if scopesEnv != "" {
		scopes = strings.Split(scopesEnv, ",")
		for i, scope := range scopes {
			scopes[i] = strings.TrimSpace(scope)
		}
	}

	ctx := context.Background()

	// Retry logic for OIDC provider initialization
	var provider *oidc.Provider
	var err error
	maxRetries := 30
	retryDelay := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		provider, err = oidc.NewProvider(ctx, issuer)
		if err == nil {
			break
		}

		if i == 0 {
			fmt.Printf("OIDC provider not ready, retrying in %v (attempt %d/%d)...\n", retryDelay, i+1, maxRetries)
		} else if i%5 == 0 {
			fmt.Printf("Still waiting for OIDC provider (attempt %d/%d)...\n", i+1, maxRetries)
		}

		time.Sleep(retryDelay)
	}

	if err != nil {
		return fmt.Errorf("failed to create OIDC provider after %d attempts: %w", maxRetries, err)
	}

	oauth2Config = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	oidcConfig := &oidc.Config{
		ClientID: clientID,
	}
	oidcVerifier = provider.Verifier(oidcConfig)
	oidcProvider = provider
	oidcEnabled = true

	return nil
}

// IsOIDCEnabled returns true if OIDC is configured and enabled
func IsOIDCEnabled() bool {
	return oidcEnabled
}

// OIDCAuthHandler initiates OIDC authentication flow
func OIDCAuthHandler(c *gin.Context) {
	if !oidcEnabled {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "OIDC not configured"})
		return
	}

	// Generate state parameter for CSRF protection
	state, err := generateState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate state"})
		return
	}

	// Store state in session cookie (secure, httponly)
	secure := !allowInsecure()
	// Use Lax mode for OAuth flow compatibility
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("oidc_state", state, 600, "/", "", secure, true) // 10 minute expiry

	// Generate nonce for additional security
	nonce, err := generateNonce()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate nonce"})
		return
	}

	// Store nonce in session cookie
	c.SetCookie("oidc_nonce", nonce, 600, "/", "", secure, true)

	// Build auth URL with state and nonce
	authURL := oauth2Config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("prompt", "select_account"),
	)

	c.Redirect(http.StatusFound, authURL)
}

// OIDCCallbackHandler handles the OIDC callback
func OIDCCallbackHandler(c *gin.Context) {
	if !oidcEnabled {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "OIDC not configured"})
		return
	}

	// Debug: Log all query parameters and cookies
	fmt.Printf("OIDC Callback - Query params: %v\n", c.Request.URL.Query())
	fmt.Printf("OIDC Callback - All cookies: %v\n", c.Request.Cookies())

	// Verify state parameter
	state := c.Query("state")
	storedState, err := c.Cookie("oidc_state")
	fmt.Printf("OIDC Callback - State from query: %s, State from cookie: %s, Cookie error: %v\n", state, storedState, err)

	if err != nil || state != storedState {
		// More detailed error for debugging
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid state parameter",
				"details": fmt.Sprintf("Cookie error: %v", err),
				"debug":   "State cookie not found or unreadable",
			})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid state parameter",
				"details": fmt.Sprintf("Expected: %s, Got: %s", storedState, state),
				"debug":   "State mismatch - possible CSRF attack or cookie issue",
			})
		}
		return
	}

	// Get nonce from cookie
	nonce, err := c.Cookie("oidc_nonce")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing nonce"})
		return
	}

	// Clear state and nonce cookies
	secure := !allowInsecure()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("oidc_state", "", -1, "/", "", secure, true)
	c.SetCookie("oidc_nonce", "", -1, "/", "", secure, true)

	// Handle error from provider
	if errMsg := c.Query("error"); errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "OIDC authentication failed",
			"details": errMsg,
		})
		return
	}

	// Exchange code for token
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing authorization code"})
		return
	}

	ctx := context.Background()
	oauth2Token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange code for token"})
		return
	}

	// Extract ID token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No ID token received"})
		return
	}

	// Verify ID token
	idToken, err := oidcVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify ID token"})
		return
	}

	// Verify nonce
	if idToken.Nonce != nonce {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid nonce"})
		return
	}

	// Extract claims
	var claims struct {
		Email             string `json:"email"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
		Subject           string `json:"sub"`
		EmailVerified     bool   `json:"email_verified"`
	}

	if err := idToken.Claims(&claims); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to extract claims"})
		return
	}

	// Determine username - prefer preferred_username, fallback to email, then subject
	username := claims.PreferredUsername
	if username == "" {
		username = claims.Email
	}
	if username == "" {
		username = claims.Subject
	}


	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No suitable username claim found"})
		return
	}

	// Handle user authentication based on mode
        if database.IsMultiUserMode() {
                if err := handleOIDCMultiUserAuth(c, username, claims.Email, claims.Name, claims.Subject); err != nil {
                        if err.Error() == "account disabled" {
                                c.JSON(http.StatusUnauthorized, gin.H{"error": "backend.auth.account_disabled"})
                        } else {
                                c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
                        }
                        return
                }
        } else {
		if err := handleOIDCSingleUserAuth(c, username); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
}

// handleOIDCMultiUserAuth handles OIDC authentication in multi-user mode
func handleOIDCMultiUserAuth(c *gin.Context, username, email, name, subject string) error {
	var user *database.User
	var err error

	user, err = database.GetUserByOIDCSubject(subject)
	if err != nil {
		user, err = database.GetUserByUsernameWithoutOIDC(username)
		if err != nil {
			user, err = database.GetUserByEmailWithoutOIDC(email)
			if err != nil {
				fmt.Printf("OIDC: No existing user found, creating new user\n")
				autoCreateUsers := config.Get("OIDC_AUTO_CREATE_USERS", "")
				if autoCreateUsers != "true" && autoCreateUsers != "1" {
					return fmt.Errorf("user not found and auto-creation disabled")
				}

			// Check if this would be the first user (for admin privileges)
			var userCount int64
			if err := database.DB.Model(&database.User{}).Count(&userCount).Error; err != nil {
				return fmt.Errorf("failed to check user count: %w", err)
			}
			firstUser := userCount == 0

			// Auto-create user using the existing CreateUser method
			userService := database.NewUserService(database.DB)
			user, err = userService.CreateUser(username, email, "", firstUser) // Empty password for OIDC users
			if err != nil {
				return fmt.Errorf("failed to create user: %w", err)
			}

			// If this is the first user, migrate single-user data asynchronously
			if firstUser {
				go func() {
					if err := database.MigrateSingleUserData(user.ID); err != nil {
						fmt.Printf("Warning: failed to migrate single-user data: %v\n", err)
					}
				}()
			}
			} else {
				fmt.Printf("OIDC: Linked user %s via email\n", user.Username)
				if err := database.DB.Model(user).Update("oidc_subject", subject).Error; err != nil {
					return fmt.Errorf("failed to link existing user to OIDC subject: %w", err)
				}
			}
		} else {
			fmt.Printf("OIDC: Linked user %s via username\n", user.Username)
			if err := database.DB.Model(user).Update("oidc_subject", subject).Error; err != nil {
				return fmt.Errorf("failed to link existing user to OIDC subject: %w", err)
			}
		}
	}

	// Update profile information from OIDC claims
	updates := make(map[string]interface{})
	if user.OidcSubject == nil || *user.OidcSubject != subject {
		updates["oidc_subject"] = subject
	}

	if email != "" && user.Email == "" {
		updates["email"] = email
	}

	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		if err := database.DB.Model(user).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update user: %w", err)
		}
	}

	// Check if user is active
	if !user.IsActive {
		return fmt.Errorf("account disabled")
	}

	// Generate JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":     user.ID.String(),
		"username":    user.Username,
		"email":       user.Email,
		"is_admin":    user.IsAdmin,
		"exp":         time.Now().Add(sessionTimeout).Unix(),
		"iat":         time.Now().Unix(),
		"iss":         "aviary",
		"aud":         "aviary-web",
		"auth_method": "oidc",
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return fmt.Errorf("failed to sign token: %w", err)
	}

	// Set secure cookie
	secure := !allowInsecure()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("auth_token", tokenString, int(sessionTimeout.Seconds()), "/", "", secure, true)

	// Redirect to frontend
	redirectURL := config.Get("OIDC_SUCCESS_REDIRECT_URL", "")
	if redirectURL == "" {
		redirectURL = "/" // Default to home page
	}
	c.Redirect(http.StatusFound, redirectURL)
	return nil
}

// handleOIDCSingleUserAuth handles OIDC authentication in single-user mode
func handleOIDCSingleUserAuth(c *gin.Context, username string) error {
	// In single-user mode, we create a JWT token for the authenticated user
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username":    username,
		"exp":         time.Now().Add(sessionTimeout).Unix(),
		"iat":         time.Now().Unix(),
		"iss":         "aviary",
		"aud":         "aviary-web",
		"auth_method": "oidc",
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return fmt.Errorf("failed to sign token: %w", err)
	}

	// Set secure cookie
	secure := !allowInsecure()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("auth_token", tokenString, int(sessionTimeout.Seconds()), "/", "", secure, true)

	// Call post-pairing callback if set (folder cache refresh)
	if GetPostPairingCallback() != nil {
		go GetPostPairingCallback()(username, true) // true = single-user mode
	}

	// Redirect to frontend
	redirectURL := config.Get("OIDC_SUCCESS_REDIRECT_URL", "")
	if redirectURL == "" {
		redirectURL = "/" // Default to home page
	}
	c.Redirect(http.StatusFound, redirectURL)
	return nil
}

// generateState generates a secure random state parameter
func generateState() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// generateNonce generates a secure random nonce
func generateNonce() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// OIDCLogoutHandler handles OIDC logout
func OIDCLogoutHandler(c *gin.Context) {
	// Clear local session
	secure := !allowInsecure()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("auth_token", "", -1, "/", "", secure, true)

	// Check if OIDC end session endpoint is available
	if oidcEnabled && oidcProvider != nil {
		var providerClaims struct {
			EndSessionEndpoint string `json:"end_session_endpoint"`
		}

		if err := oidcProvider.Claims(&providerClaims); err == nil && providerClaims.EndSessionEndpoint != "" {
			// Redirect to OIDC provider logout
			logoutURL := providerClaims.EndSessionEndpoint

			// Add post logout redirect URI if configured
			if postLogoutRedirect := config.Get("OIDC_POST_LOGOUT_REDIRECT_URL", ""); postLogoutRedirect != "" {
				logoutURL += "?post_logout_redirect_uri=" + url.QueryEscape(postLogoutRedirect)
			}

			c.Redirect(http.StatusFound, logoutURL)
			return
		}
	}

	// Fallback to local logout
	c.JSON(http.StatusOK, gin.H{"success": true})
}
