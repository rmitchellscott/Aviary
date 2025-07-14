package auth

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/database"
)

// CreateAPIKeyRequest represents an API key creation request
type CreateAPIKeyRequest struct {
	Name      string `json:"name" binding:"required,min=1,max=100"`
	ExpiresAt *int64 `json:"expires_at,omitempty"` // Unix timestamp, optional
}

// APIKeyResponse represents an API key in responses
type APIKeyResponse struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	KeyPrefix string     `json:"key_prefix"`
	IsActive  bool       `json:"is_active"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// CreateAPIKeyResponse includes the full API key (only returned once)
type CreateAPIKeyResponse struct {
	APIKeyResponse
	APIKey string `json:"api_key"`
}

// CreateAPIKeyHandler creates a new API key for the current user
func CreateAPIKeyHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key management not available in single-user mode"})
		return
	}

	user, ok := RequireUser(c)
	if !ok {
		return
	}

	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		expiry := time.Unix(*req.ExpiresAt, 0)
		if expiry.Before(time.Now()) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Expiration time must be in the future"})
			return
		}
		expiresAt = &expiry
	}

	apiKeyService := database.NewAPIKeyService(database.DB)
	apiKey, keyString, err := apiKeyService.GenerateAPIKey(user.ID, req.Name, expiresAt)
	if err != nil {
		if err.Error() == "maximum number of API keys (10) reached" {
			c.JSON(http.StatusConflict, gin.H{"error": "Maximum number of API keys reached"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create API key"})
		return
	}

	response := CreateAPIKeyResponse{
		APIKeyResponse: APIKeyResponse{
			ID:        apiKey.ID,
			Name:      apiKey.Name,
			KeyPrefix: apiKey.KeyPrefix,
			IsActive:  apiKey.IsActive,
			LastUsed:  apiKey.LastUsed,
			ExpiresAt: apiKey.ExpiresAt,
			CreatedAt: apiKey.CreatedAt,
		},
		APIKey: keyString,
	}

	c.JSON(http.StatusCreated, response)
}

// GetAPIKeysHandler returns all API keys for the current user
func GetAPIKeysHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key management not available in single-user mode"})
		return
	}

	user, ok := RequireUser(c)
	if !ok {
		return
	}

	// Check if user wants only active keys
	activeOnly := c.Query("active") == "true"

	apiKeyService := database.NewAPIKeyService(database.DB)
	var apiKeys []database.APIKey
	var err error

	if activeOnly {
		apiKeys, err = apiKeyService.GetActiveUserAPIKeys(user.ID)
	} else {
		apiKeys, err = apiKeyService.GetUserAPIKeys(user.ID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve API keys"})
		return
	}

	// Convert to response format
	response := make([]APIKeyResponse, len(apiKeys))
	for i, key := range apiKeys {
		response[i] = APIKeyResponse{
			ID:        key.ID,
			Name:      key.Name,
			KeyPrefix: key.KeyPrefix,
			IsActive:  key.IsActive,
			LastUsed:  key.LastUsed,
			ExpiresAt: key.ExpiresAt,
			CreatedAt: key.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetAPIKeyHandler returns a specific API key
func GetAPIKeyHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key management not available in single-user mode"})
		return
	}

	user, ok := RequireUser(c)
	if !ok {
		return
	}

	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key ID"})
		return
	}

	apiKeyService := database.NewAPIKeyService(database.DB)
	apiKey, err := apiKeyService.GetAPIKeyByID(keyID, user.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	response := APIKeyResponse{
		ID:        apiKey.ID,
		Name:      apiKey.Name,
		KeyPrefix: apiKey.KeyPrefix,
		IsActive:  apiKey.IsActive,
		LastUsed:  apiKey.LastUsed,
		ExpiresAt: apiKey.ExpiresAt,
		CreatedAt: apiKey.CreatedAt,
	}

	c.JSON(http.StatusOK, response)
}

// UpdateAPIKeyHandler updates an API key (name only)
func UpdateAPIKeyHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key management not available in single-user mode"})
		return
	}

	user, ok := RequireUser(c)
	if !ok {
		return
	}

	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key ID"})
		return
	}

	var req struct {
		Name string `json:"name" binding:"required,min=1,max=100"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	apiKeyService := database.NewAPIKeyService(database.DB)
	if err := apiKeyService.UpdateAPIKeyName(keyID, user.ID, req.Name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeactivateAPIKeyHandler deactivates an API key
func DeactivateAPIKeyHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key management not available in single-user mode"})
		return
	}

	user, ok := RequireUser(c)
	if !ok {
		return
	}

	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key ID"})
		return
	}

	apiKeyService := database.NewAPIKeyService(database.DB)
	if err := apiKeyService.DeactivateAPIKey(keyID, user.ID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteAPIKeyHandler permanently deletes an API key
func DeleteAPIKeyHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key management not available in single-user mode"})
		return
	}

	user, ok := RequireUser(c)
	if !ok {
		return
	}

	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key ID"})
		return
	}

	apiKeyService := database.NewAPIKeyService(database.DB)
	if err := apiKeyService.DeleteAPIKey(keyID, user.ID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetAPIKeyStatsHandler returns API key statistics (admin only)
func GetAPIKeyStatsHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key management not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	apiKeyService := database.NewAPIKeyService(database.DB)
	stats, err := apiKeyService.GetAPIKeyStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve API key statistics"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// CleanupExpiredAPIKeysHandler removes expired API keys (admin only)
func CleanupExpiredAPIKeysHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key management not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	apiKeyService := database.NewAPIKeyService(database.DB)
	if err := apiKeyService.CleanupExpiredAPIKeys(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cleanup expired API keys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Expired API keys have been cleaned up"})
}

// GetAllAPIKeysHandler returns all API keys in the system (admin only)
func GetAllAPIKeysHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key management not available in single-user mode"})
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

	offset := (page - 1) * limit

	// Get API keys with pagination
	var apiKeys []database.APIKey
	var total int64

	query := database.DB.Model(&database.APIKey{})
	
	// Count total
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count API keys"})
		return
	}

	// Get paginated results with user info
	if err := query.Preload("User").Offset(offset).Limit(limit).Order("created_at DESC").Find(&apiKeys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve API keys"})
		return
	}

	// Convert to response format
	type AdminAPIKeyResponse struct {
		APIKeyResponse
		UserID   uuid.UUID `json:"user_id"`
		Username string    `json:"username"`
	}

	response := make([]AdminAPIKeyResponse, len(apiKeys))
	for i, key := range apiKeys {
		response[i] = AdminAPIKeyResponse{
			APIKeyResponse: APIKeyResponse{
				ID:        key.ID,
				Name:      key.Name,
				KeyPrefix: key.KeyPrefix,
				IsActive:  key.IsActive,
				LastUsed:  key.LastUsed,
				ExpiresAt: key.ExpiresAt,
				CreatedAt: key.CreatedAt,
			},
			UserID:   key.UserID,
			Username: key.User.Username,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"api_keys":     response,
		"total":        total,
		"page":         page,
		"limit":        limit,
		"total_pages":  (total + int64(limit) - 1) / int64(limit),
	})
}