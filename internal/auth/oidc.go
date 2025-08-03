package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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
	"github.com/rmitchellscott/aviary/internal/logging"
	"golang.org/x/oauth2"
)

var (
	oidcProvider *oidc.Provider
	oauth2Config *oauth2.Config
	oidcVerifier *oidc.IDTokenVerifier
	oidcEnabled  bool
)

// oidcDebugLog logs OIDC debug messages if OIDC_DEBUG is enabled
func oidcDebugLog(format string, v ...interface{}) {
	if config.Get("OIDC_DEBUG", "") == "true" || config.Get("OIDC_DEBUG", "") == "1" {
		logging.Logf("[OIDC DEBUG] "+format, v...)
	}
}

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

	logging.Logf("[STARTUP] Initializing OIDC provider with issuer: %s", issuer)

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

		logging.Logf("[STARTUP] OIDC provider not ready, retrying in %v (attempt %d/%d)...", retryDelay, i+1, maxRetries)

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

	logging.Logf("[STARTUP] OIDC provider initialized successfully")

	return nil
}

// IsOIDCEnabled returns true if OIDC is configured and enabled
func IsOIDCEnabled() bool {
	return oidcEnabled
}

// IsOIDCSsoOnlyEnabled returns true if OIDC SSO-only mode is enabled
func IsOIDCSsoOnlyEnabled() bool {
	if !oidcEnabled {
		return false
	}
	ssoOnly := config.Get("OIDC_SSO_ONLY", "")
	return ssoOnly == "true" || ssoOnly == "1"
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

	oidcDebugLog("OIDC callback handler started")

	// Debug: Log all query parameters and cookies
	oidcDebugLog("OIDC Callback - Query params: %v", c.Request.URL.Query())
	oidcDebugLog("OIDC Callback - All cookies: %v", c.Request.Cookies())

	// Verify state parameter
	state := c.Query("state")
	storedState, err := c.Cookie("oidc_state")
	oidcDebugLog("OIDC Callback - State from query: %s, State from cookie: %s, Cookie error: %v", state, storedState, err)

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

	// Extract raw claims first for debug logging
	var rawClaims map[string]interface{}
	if err := idToken.Claims(&rawClaims); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to extract raw claims"})
		return
	}

	// Debug log raw claims from OIDC provider
	if rawClaimsJSON, err := json.MarshalIndent(rawClaims, "", "  "); err == nil {
		oidcDebugLog("Raw claims from OIDC provider:\n%s", string(rawClaimsJSON))
	} else {
		oidcDebugLog("Raw claims from OIDC provider (failed to serialize): %+v", rawClaims)
	}

	// Extract claims into our structured format
	var claims struct {
		Email             string   `json:"email"`
		Name              string   `json:"name"`
		PreferredUsername string   `json:"preferred_username"`
		Subject           string   `json:"sub"`
		EmailVerified     bool     `json:"email_verified"`
		Groups            []string `json:"groups"`
	}

	if err := idToken.Claims(&claims); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to extract claims"})
		return
	}

	// Debug log parsed claims
	if claimsJSON, err := json.Marshal(claims); err == nil {
		oidcDebugLog("Parsed claims: %s", string(claimsJSON))
	} else {
		oidcDebugLog("Parsed claims (failed to serialize): email=%s, name=%s, preferred_username=%s, subject=%s, email_verified=%t, groups=%v",
			claims.Email, claims.Name, claims.PreferredUsername, claims.Subject, claims.EmailVerified, claims.Groups)
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

	// OIDC authentication requires multi-user mode
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "OIDC authentication requires multi-user mode"})
		return
	}

	// Handle user authentication in multi-user mode
	if err := handleOIDCMultiUserAuth(c, username, claims.Email, claims.Name, claims.Subject, claims.Groups, rawIDToken); err != nil {
		if err.Error() == "account disabled" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "backend.auth.account_disabled"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
}

// handleOIDCMultiUserAuth handles OIDC authentication in multi-user mode
func handleOIDCMultiUserAuth(c *gin.Context, username, email, name, subject string, groups []string, rawIDToken string) error {
	var user *database.User
	var err error

	oidcDebugLog("Starting multi-user authentication for subject: %s, username: %s, email: %s", subject, username, email)

	user, err = database.GetUserByOIDCSubject(subject)
	if err != nil {
		oidcDebugLog("No user found with OIDC subject %s, trying username/email lookup", subject)
		user, err = database.GetUserByUsernameWithoutOIDC(username)
		if err != nil {
			oidcDebugLog("No user found with username %s, trying email lookup", username)
			user, err = database.GetUserByEmailWithoutOIDC(email)
			if err != nil {
				oidcDebugLog("No existing user found for username %s or email %s, checking auto-creation", username, email)
				autoCreateUsers := config.Get("OIDC_AUTO_CREATE_USERS", "")
				oidcDebugLog("OIDC_AUTO_CREATE_USERS setting: %s", autoCreateUsers)
				if autoCreateUsers != "true" && autoCreateUsers != "1" {
					oidcDebugLog("Auto-creation disabled, rejecting user creation")
					return fmt.Errorf("user not found and auto-creation disabled")
				}

				// Check if this would be the first user (for admin privileges)
				var userCount int64
				if err := database.DB.Model(&database.User{}).Count(&userCount).Error; err != nil {
					return fmt.Errorf("failed to check user count: %w", err)
				}
				firstUser := userCount == 0
				oidcDebugLog("User count: %d, firstUser: %t", userCount, firstUser)

				// Determine admin status based on OIDC groups or first user
				isAdmin := shouldBeAdminFromGroups(groups, firstUser)
				oidcDebugLog("Determined admin status for new user: %t", isAdmin)

				// Check if this would be the first admin user
				var adminCount int64
				if err := database.DB.Model(&database.User{}).Where("is_admin = ?", true).Count(&adminCount).Error; err != nil {
					return fmt.Errorf("failed to check admin count: %w", err)
				}
				firstAdminUser := adminCount == 0

				// Auto-create user using the existing CreateUser method
				userService := database.NewUserService(database.DB)
				oidcDebugLog("Creating new user with username: %s, email: %s, admin: %t", username, email, isAdmin)
				user, err = userService.CreateUser(username, email, "", isAdmin) // Empty password for OIDC users
				if err != nil {
					oidcDebugLog("Failed to create user: %v", err)
					return fmt.Errorf("failed to create user: %w", err)
				}
				oidcDebugLog("Successfully created new user with ID: %s", user.ID)

				// If this is the first admin user, migrate single-user data asynchronously
				if firstAdminUser && isAdmin {
					oidcDebugLog("First admin user created, migrating single-user data to user ID: %s", user.ID)
					go func() {
						if err := database.MigrateSingleUserData(user.ID); err != nil {
							oidcDebugLog("Warning: failed to migrate single-user data: %v", err)
						}
					}()
				}
			} else {
				oidcDebugLog("Found existing user %s via email, linking to OIDC subject %s", user.Username, subject)
				if err := database.DB.Model(user).Update("oidc_subject", subject).Error; err != nil {
					oidcDebugLog("Failed to link existing user to OIDC subject: %v", err)
					return fmt.Errorf("failed to link existing user to OIDC subject: %w", err)
				}
				oidcDebugLog("Successfully linked user %s to OIDC subject", user.Username)
			}
		} else {
			oidcDebugLog("Found existing user %s via username, linking to OIDC subject %s", user.Username, subject)
			if err := database.DB.Model(user).Update("oidc_subject", subject).Error; err != nil {
				oidcDebugLog("Failed to link existing user to OIDC subject: %v", err)
				return fmt.Errorf("failed to link existing user to OIDC subject: %w", err)
			}
			oidcDebugLog("Successfully linked user %s to OIDC subject", user.Username)
		}
	} else {
		oidcDebugLog("Found existing user %s with OIDC subject %s", user.Username, subject)
	}

	// Update profile information from OIDC claims
	updates := make(map[string]interface{})
	if user.OidcSubject == nil || *user.OidcSubject != subject {
		updates["oidc_subject"] = subject
	}

	// Always sync email from OIDC provider
	if email != "" {
		updates["email"] = email
	}

	// Always sync username from OIDC provider (with conflict handling)
	if username != "" && username != user.Username {
		// Check if username already exists for another user
		var existingUser database.User
		if err := database.DB.Where("username = ? AND id != ?", username, user.ID).First(&existingUser).Error; err == nil {
			// Username taken - log warning but continue login
			oidcDebugLog("[OIDC] Warning: Cannot update username to '%s' for user %s - already taken by user %s", username, user.ID, existingUser.ID)
		} else {
			// Safe to update
			oidcDebugLog("[OIDC] Updating username from '%s' to '%s' for user %s", user.Username, username, user.ID)
			updates["username"] = username
		}
	}

	// Update admin status based on group membership if enabled
	if IsOIDCGroupBasedAdminEnabled() {
		oidcDebugLog("OIDC group-based admin is enabled, checking current admin status")
		currentAdminStatus := shouldBeAdminFromGroups(groups, false)
		oidcDebugLog("Current user admin status: %t, calculated admin status: %t", user.IsAdmin, currentAdminStatus)
		if user.IsAdmin != currentAdminStatus {
			oidcDebugLog("Admin status change detected: %t -> %t", user.IsAdmin, currentAdminStatus)
			updates["is_admin"] = currentAdminStatus
		} else {
			oidcDebugLog("No admin status change needed")
		}
	} else {
		oidcDebugLog("OIDC group-based admin is disabled, preserving existing admin status: %t", user.IsAdmin)
	}

	// Always update last login for OIDC authentication
	now := time.Now()
	updates["last_login"] = &now

	if len(updates) > 0 {
		oidcDebugLog("Applying user updates: %+v", updates)
		updates["updated_at"] = now
		if err := database.DB.Model(user).Updates(updates).Error; err != nil {
			oidcDebugLog("Failed to update user: %v", err)
			return fmt.Errorf("failed to update user: %w", err)
		}
		// Refresh user object to reflect updates
		if err := database.DB.First(user, user.ID).Error; err != nil {
			return fmt.Errorf("failed to refresh user: %w", err)
		}
		oidcDebugLog("Successfully updated user")
	} else {
		oidcDebugLog("No user updates needed")
	}

	// Check if user is active
	oidcDebugLog("Checking if user %s is active: %t", user.Username, user.IsActive)
	if !user.IsActive {
		oidcDebugLog("User account is disabled, rejecting authentication")
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

	// Store ID token for logout (in a separate cookie)
	c.SetCookie("oidc_id_token", rawIDToken, int(sessionTimeout.Seconds()), "/", "", secure, true)

	// Redirect to frontend
	redirectURL := config.Get("OIDC_SUCCESS_REDIRECT_URL", "")
	if redirectURL == "" {
		redirectURL = "/" // Default to home page
	}
	oidcDebugLog("OIDC authentication successful for user %s (ID: %s, admin: %t), redirecting to: %s", user.Username, user.ID, user.IsAdmin, redirectURL)
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

	// Get stored ID token for logout hint
	idToken, _ := c.Cookie("oidc_id_token")
	// Clear ID token cookie
	c.SetCookie("oidc_id_token", "", -1, "/", "", secure, true)

	// Check if OIDC end session endpoint is available
	if oidcEnabled && oidcProvider != nil {
		var providerClaims struct {
			EndSessionEndpoint string `json:"end_session_endpoint"`
		}

		if err := oidcProvider.Claims(&providerClaims); err == nil && providerClaims.EndSessionEndpoint != "" {
			// Redirect to OIDC provider logout
			logoutURL := providerClaims.EndSessionEndpoint

			// Build query parameters
			params := url.Values{}

			// Add ID token hint if available
			if idToken != "" {
				params.Add("id_token_hint", idToken)
			}

			// Add post logout redirect URI if configured
			if postLogoutRedirect := config.Get("OIDC_POST_LOGOUT_REDIRECT_URL", ""); postLogoutRedirect != "" {
				params.Add("post_logout_redirect_uri", postLogoutRedirect)
			}

			if len(params) > 0 {
				logoutURL += "?" + params.Encode()
			}

			c.Redirect(http.StatusFound, logoutURL)
			return
		}
	}

	// Fallback to local logout
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func shouldBeAdminFromGroups(groups []string, isFirstUser bool) bool {
	adminGroup := config.Get("OIDC_ADMIN_GROUP", "")

	oidcDebugLog("Checking admin group membership - configured admin group: '%s', user groups: %v, isFirstUser: %t", adminGroup, groups, isFirstUser)

	if adminGroup == "" {
		oidcDebugLog("No admin group configured, using isFirstUser logic: %t", isFirstUser)
		return isFirstUser
	}

	for _, group := range groups {
		if group == adminGroup {
			oidcDebugLog("User found in admin group '%s' - granting admin privileges", adminGroup)
			return true
		}
	}

	oidcDebugLog("User not found in admin group '%s' - denying admin privileges", adminGroup)
	return false
}

func IsOIDCGroupBasedAdminEnabled() bool {
	return config.Get("OIDC_ADMIN_GROUP", "") != ""
}

// GetOIDCButtonText returns the custom button text for OIDC login, or empty string to use i18n fallback
func GetOIDCButtonText() string {
	return config.Get("OIDC_BUTTON_TEXT", "")
}
