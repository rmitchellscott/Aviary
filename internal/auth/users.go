package auth

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/database"
)

// PostPairingCallback is called after successful pairing
type PostPairingCallback func(userID string, singleUserMode bool)

// Global callback for post-pairing actions
var postPairingCallback PostPairingCallback

// SetPostPairingCallback sets the callback to be called after successful pairing
func SetPostPairingCallback(callback PostPairingCallback) {
	postPairingCallback = callback
}

// GetPostPairingCallback returns the current post-pairing callback
func GetPostPairingCallback() PostPairingCallback {
	return postPairingCallback
}

// UpdateUserRequest represents a user update request
type UpdateUserRequest struct {
	Email            *string  `json:"email,omitempty" binding:"omitempty,email"`
	RmapiHost        *string  `json:"rmapi_host,omitempty"`
	DefaultRmdir     *string  `json:"default_rmdir,omitempty"`
	CoverpageSetting *string  `json:"coverpage_setting,omitempty"`
	PageResolution   *string  `json:"page_resolution,omitempty"`
	PageDPI          *float64 `json:"page_dpi,omitempty"`
	IsAdmin          *bool    `json:"is_admin,omitempty"`
	IsActive         *bool    `json:"is_active,omitempty"`
}

// UpdatePasswordRequest represents a password update request
type UpdatePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

// AdminUpdatePasswordRequest represents an admin password update request
type AdminUpdatePasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// AdminResetPasswordRequest represents an admin password reset request
type AdminResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// SelfDeleteRequest represents a self-service account deletion request
type SelfDeleteRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	Confirmation    string `json:"confirmation" binding:"required"`
}

func isUserPaired(id uuid.UUID) bool {
	// In DRY_RUN mode, always consider users as paired
	if config.Get("DRY_RUN", "") != "" {
		return true
	}

	baseDir := config.Get("DATA_DIR", "")
	if baseDir == "" {
		baseDir = "/data"
	}
	cfgPath := filepath.Join(baseDir, "users", id.String(), "rmapi", "rmapi.conf")
	info, err := os.Stat(cfgPath)
	return err == nil && info.Size() > 0
}

// filterEnv removes environment variables with the given prefix from the slice
func filterEnv(env []string, prefix string) []string {
	var filtered []string
	for _, e := range env {
		if !strings.HasPrefix(e, prefix+"=") {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// GetUsersHandler returns all users (admin only)
func GetUsersHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "User management not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	// Parse query parameters
	page := 1
	limit := 50
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	search := c.Query("search")
	activeOnly := c.Query("active") == "true"

	offset := (page - 1) * limit

	// Build query
	query := database.DB.Model(&database.User{})

	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	if search != "" {
		query = query.Where("username ILIKE ? OR email ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count users"})
		return
	}

	// Get paginated results
	var users []database.User
	if err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve users"})
		return
	}

	// Convert to response format
	response := make([]UserResponse, len(users))
	for i, user := range users {
		response[i] = UserResponse{
			ID:               user.ID,
			Username:         user.Username,
			Email:            user.Email,
			IsAdmin:          user.IsAdmin,
			IsActive:         user.IsActive,
			RmapiHost:        user.RmapiHost,
			RmapiPaired:      isUserPaired(user.ID),
			DefaultRmdir:     user.DefaultRmdir,
			CoverpageSetting: user.CoverpageSetting,
			CreatedAt:        user.CreatedAt,
			LastLogin:        user.LastLogin,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"users":       response,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": (total + int64(limit) - 1) / int64(limit),
	})
}

// GetUserHandler returns a specific user (admin only)
func GetUserHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "User management not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	userService := database.NewUserService(database.DB)
	user, err := userService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get user statistics
	stats, err := userService.GetUserStats(userID)
	if err != nil {
		stats = make(map[string]interface{})
	}

	response := UserResponse{
		ID:               user.ID,
		Username:         user.Username,
		Email:            user.Email,
		IsAdmin:          user.IsAdmin,
		IsActive:         user.IsActive,
		RmapiHost:        user.RmapiHost,
		RmapiPaired:      isUserPaired(user.ID),
		DefaultRmdir:     user.DefaultRmdir,
		CoverpageSetting: user.CoverpageSetting,
		CreatedAt:        user.CreatedAt,
		LastLogin:        user.LastLogin,
	}

	c.JSON(http.StatusOK, gin.H{
		"user":  response,
		"stats": stats,
	})
}

// UpdateUserHandler updates a user (admin only)
func UpdateUserHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "User management not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": validationErrorMessage(err)})
		return
	}

	// Build update map
	updates := make(map[string]interface{})
	if req.Email != nil && *req.Email != "" {
		updates["email"] = *req.Email
	}
	if req.RmapiHost != nil {
		updates["rmapi_host"] = *req.RmapiHost
	}
	if req.DefaultRmdir != nil && *req.DefaultRmdir != "" {
		updates["default_rmdir"] = *req.DefaultRmdir
	}
	if req.CoverpageSetting != nil && *req.CoverpageSetting != "" {
		updates["coverpage_setting"] = *req.CoverpageSetting
	}
	if req.PageResolution != nil {
		updates["page_resolution"] = *req.PageResolution
	}
	if req.PageDPI != nil {
		updates["page_dpi"] = *req.PageDPI
	}
	if req.IsAdmin != nil {
		updates["is_admin"] = *req.IsAdmin
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	userService := database.NewUserService(database.DB)
	if err := userService.UpdateUserSettings(userID, updates); err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteCurrentUserHandler allows users to delete their own account
func DeleteCurrentUserHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "User management not available in single-user mode"})
		return
	}

	user, ok := RequireUser(c)
	if !ok {
		return
	}

	var req SelfDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": validationErrorMessage(err)})
		return
	}

	// Require confirmation text to prevent accidental deletions
	if req.Confirmation != "DELETE MY ACCOUNT" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Confirmation text must be 'DELETE MY ACCOUNT'"})
		return
	}

	// Verify current password
	userService := database.NewUserService(database.DB)
	_, err := userService.AuthenticateUser(user.Username, req.CurrentPassword)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
		return
	}

	// Delete the user
	if err := userService.DeleteUser(user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete account"})
		return
	}

	// Clear the session cookie
	secure := !allowInsecure()
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("auth_token", "", -1, "/", "", secure, true)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// UpdateCurrentUserHandler updates the current user's profile
func UpdateCurrentUserHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "User management not available in single-user mode"})
		return
	}

	user, ok := RequireUser(c)
	if !ok {
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": validationErrorMessage(err)})
		return
	}

	// Build update map (non-admin users can't change admin/active status)
	updates := make(map[string]interface{})

	// Update fields only if they were provided in the request (pointers allow us to detect this)
	if req.Email != nil && *req.Email != "" {
		updates["email"] = *req.Email
	}

	if req.RmapiHost != nil {
		updates["rmapi_host"] = *req.RmapiHost // Allow clearing by setting to empty string
	}

	if req.DefaultRmdir != nil && *req.DefaultRmdir != "" {
		updates["default_rmdir"] = *req.DefaultRmdir
	}

	if req.CoverpageSetting != nil && *req.CoverpageSetting != "" {
		updates["coverpage_setting"] = *req.CoverpageSetting
	}

	if req.PageResolution != nil {
		updates["page_resolution"] = *req.PageResolution // Allow clearing by setting to empty string
	}

	if req.PageDPI != nil {
		updates["page_dpi"] = *req.PageDPI // Allow clearing by setting to 0
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	userService := database.NewUserService(database.DB)
	if err := userService.UpdateUserSettings(user.ID, updates); err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// UpdatePasswordHandler updates the current user's password
func UpdatePasswordHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "User management not available in single-user mode"})
		return
	}

	user, ok := RequireUser(c)
	if !ok {
		return
	}

	var req UpdatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": validationErrorMessage(err)})
		return
	}

	// Verify current password
	userService := database.NewUserService(database.DB)
	_, err := userService.AuthenticateUser(user.Username, req.CurrentPassword)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
		return
	}

	// Update password
	if err := userService.UpdateUserPassword(user.ID, req.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// PairRMAPIHandler pairs the current user with rmapi using a one-time code.
func PairRMAPIHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not available in single-user mode"})
		return
	}

	// Mock successful pairing in DRY_RUN mode
	if config.Get("DRY_RUN", "") != "" {
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	user, ok := RequireUser(c)
	if !ok {
		return
	}

	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var cfgPath string
	baseDir := config.Get("DATA_DIR", "")
	if baseDir == "" {
		baseDir = "/data"
	}
	cfgDir := filepath.Join(baseDir, "users", user.ID.String(), "rmapi")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to determine config path"})
		return
	}
	cfgPath = filepath.Join(cfgDir, "rmapi.conf")

	cmd := exec.Command("rmapi", "cd")
	cmd.Stdin = strings.NewReader(req.Code + "\n")
	env := os.Environ()
	env = append(env, "RMAPI_CONFIG="+cfgPath)
	if user.RmapiHost != "" {
		env = append(env, "RMAPI_HOST="+user.RmapiHost)
	} else {
		// Remove server-level RMAPI_HOST to use official cloud
		env = filterEnv(env, "RMAPI_HOST")
	}
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Pairing failed"})
		return
	}

	// Call post-pairing callback if set (async for folder cache refresh)
	if postPairingCallback != nil {
		go postPairingCallback(user.ID.String(), false) // false = multi-user mode
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// UnpairRMAPIHandler removes the rmapi configuration for the current user.
func UnpairRMAPIHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not available in single-user mode"})
		return
	}

	user, ok := RequireUser(c)
	if !ok {
		return
	}

	baseDir := config.Get("DATA_DIR", "")
	if baseDir == "" {
		baseDir = "/data"
	}
	cfgPath := filepath.Join(baseDir, "users", user.ID.String(), "rmapi", "rmapi.conf")
	_ = os.Remove(cfgPath)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// AdminUpdatePasswordHandler updates any user's password (admin only)
func AdminUpdatePasswordHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "User management not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req AdminUpdatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": validationErrorMessage(err)})
		return
	}

	userService := database.NewUserService(database.DB)
	if err := userService.UpdateUserPassword(userID, req.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeactivateUserHandler deactivates a user (admin only)
func DeactivateUserHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "User management not available in single-user mode"})
		return
	}

	currentUser, ok := RequireAdmin(c)
	if !ok {
		return
	}

	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Prevent admin from deactivating themselves
	if currentUser.ID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot deactivate yourself"})
		return
	}

	userService := database.NewUserService(database.DB)
	if err := userService.DeactivateUser(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deactivate user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ActivateUserHandler activates a user (admin only)
func ActivateUserHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "User management not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	userService := database.NewUserService(database.DB)
	if err := userService.ActivateUser(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to activate user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetUserStatsHandler returns user statistics (admin only)
func GetUserStatsHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "User management not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	stats, err := database.GetDatabaseStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user statistics"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetCurrentUserStatsHandler returns current user's statistics
func GetCurrentUserStatsHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "User management not available in single-user mode"})
		return
	}

	user, ok := RequireUser(c)
	if !ok {
		return
	}

	userService := database.NewUserService(database.DB)
	stats, err := userService.GetUserStats(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user statistics"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// DeleteUserHandler deletes a user (admin only)
func DeleteUserHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "User management not available in single-user mode"})
		return
	}

	currentUser, ok := RequireAdmin(c)
	if !ok {
		return
	}

	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Prevent admin from deleting themselves
	if currentUser.ID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete yourself"})
		return
	}

	userService := database.NewUserService(database.DB)
	if err := userService.DeleteUser(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// AdminResetPasswordHandler resets any user's password (admin only)
func AdminResetPasswordHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "User management not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req AdminResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": validationErrorMessage(err)})
		return
	}

	userService := database.NewUserService(database.DB)
	if err := userService.UpdateUserPassword(userID, req.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// PromoteUserHandler promotes a user to admin (admin only)
func PromoteUserHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "User management not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	userService := database.NewUserService(database.DB)
	updates := map[string]interface{}{
		"is_admin": true,
	}

	if err := userService.UpdateUserSettings(userID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to promote user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DemoteUserHandler demotes an admin to user (admin only)
func DemoteUserHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "User management not available in single-user mode"})
		return
	}

	currentUser, ok := RequireAdmin(c)
	if !ok {
		return
	}

	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Prevent admin from demoting themselves
	if currentUser.ID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot demote yourself"})
		return
	}

	userService := database.NewUserService(database.DB)
	updates := map[string]interface{}{
		"is_admin": false,
	}

	if err := userService.UpdateUserSettings(userID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to demote user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
