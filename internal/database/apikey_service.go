package database

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// APIKeyService provides API key-related database operations
type APIKeyService struct {
	db *gorm.DB
}

// NewAPIKeyService creates a new API key service
func NewAPIKeyService(db *gorm.DB) *APIKeyService {
	return &APIKeyService{db: db}
}

// GenerateAPIKey creates a new API key for a user
func (s *APIKeyService) GenerateAPIKey(userID uuid.UUID, name string, expiresAt *time.Time) (*APIKey, string, error) {
	// Check if user has reached the maximum number of API keys
	maxKeysStr, err := GetSystemSetting("max_api_keys_per_user")
	if err != nil {
		maxKeysStr = "10" // Default fallback
	}
	
	maxKeys, err := strconv.Atoi(maxKeysStr)
	if err != nil {
		maxKeys = 10
	}
	
	var existingCount int64
	if err := s.db.Model(&APIKey{}).Where("user_id = ? AND is_active = ?", userID, true).Count(&existingCount).Error; err != nil {
		return nil, "", fmt.Errorf("failed to count existing API keys: %w", err)
	}
	
	if existingCount >= int64(maxKeys) {
		return nil, "", fmt.Errorf("maximum number of API keys (%d) reached", maxKeys)
	}
	
	// Generate a random API key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, "", fmt.Errorf("failed to generate API key: %w", err)
	}
	
	// Create the API key string with a prefix for easy identification
	apiKey := fmt.Sprintf("aviary_%x", keyBytes)
	
	// Hash the API key for storage
	hashedKey, err := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash API key: %w", err)
	}
	
	// Create the API key record
	apiKeyRecord := &APIKey{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		KeyHash:   string(hashedKey),
		KeyPrefix: apiKey[:16], // Store first 16 chars for display
		IsActive:  true,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
	
	if err := s.db.Create(apiKeyRecord).Error; err != nil {
		return nil, "", fmt.Errorf("failed to create API key: %w", err)
	}
	
	// Return the API key record and the actual key (only time it's returned)
	return apiKeyRecord, apiKey, nil
}

// ValidateAPIKey validates an API key and returns the associated user
func (s *APIKeyService) ValidateAPIKey(apiKey string) (*User, error) {
	if !strings.HasPrefix(apiKey, "aviary_") {
		return nil, errors.New("invalid API key format")
	}
	
	// Find all active API keys and check each one
	var apiKeys []APIKey
	if err := s.db.Where("is_active = ? AND (expires_at IS NULL OR expires_at > ?)", 
		true, time.Now()).Find(&apiKeys).Error; err != nil {
		return nil, fmt.Errorf("failed to query API keys: %w", err)
	}
	
	for _, key := range apiKeys {
		if err := bcrypt.CompareHashAndPassword([]byte(key.KeyHash), []byte(apiKey)); err == nil {
			// Update last used timestamp
			s.db.Model(&key).Update("last_used", time.Now())
			
			// Get and return the user
			var user User
			if err := s.db.Where("id = ? AND is_active = ?", key.UserID, true).First(&user).Error; err != nil {
				return nil, fmt.Errorf("failed to get user: %w", err)
			}
			
			return &user, nil
		}
	}
	
	return nil, errors.New("invalid API key")
}

// ValidateAPIKeyConstantTime validates an API key with constant time comparison
func (s *APIKeyService) ValidateAPIKeyConstantTime(providedKey string) (*User, error) {
	if !strings.HasPrefix(providedKey, "aviary_") {
		return nil, errors.New("invalid API key format")
	}
	
	// Get the prefix to narrow down the search
	prefix := ""
	if len(providedKey) >= 16 {
		prefix = providedKey[:16]
	}
	
	var apiKeys []APIKey
	query := s.db.Where("is_active = ? AND (expires_at IS NULL OR expires_at > ?)", true, time.Now())
	if prefix != "" {
		query = query.Where("key_prefix = ?", prefix)
	}
	
	if err := query.Find(&apiKeys).Error; err != nil {
		return nil, fmt.Errorf("failed to query API keys: %w", err)
	}
	
	var foundUser *User
	validKey := false
	
	// Check all keys with constant time comparison
	for _, key := range apiKeys {
		if err := bcrypt.CompareHashAndPassword([]byte(key.KeyHash), []byte(providedKey)); err == nil {
			if !validKey { // Only set once to maintain constant time
				validKey = true
				
				// Update last used timestamp
				s.db.Model(&key).Update("last_used", time.Now())
				
				// Get the user
				var user User
				if err := s.db.Where("id = ? AND is_active = ?", key.UserID, true).First(&user).Error; err == nil {
					foundUser = &user
				}
			}
		}
	}
	
	if !validKey || foundUser == nil {
		return nil, errors.New("invalid API key")
	}
	
	return foundUser, nil
}

// GetUserAPIKeys retrieves all API keys for a user
func (s *APIKeyService) GetUserAPIKeys(userID uuid.UUID) ([]APIKey, error) {
	var apiKeys []APIKey
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&apiKeys).Error; err != nil {
		return nil, err
	}
	return apiKeys, nil
}

// GetActiveUserAPIKeys retrieves all active API keys for a user
func (s *APIKeyService) GetActiveUserAPIKeys(userID uuid.UUID) ([]APIKey, error) {
	var apiKeys []APIKey
	if err := s.db.Where("user_id = ? AND is_active = ? AND (expires_at IS NULL OR expires_at > ?)", 
		userID, true, time.Now()).Order("created_at DESC").Find(&apiKeys).Error; err != nil {
		return nil, err
	}
	return apiKeys, nil
}

// DeactivateAPIKey deactivates an API key
func (s *APIKeyService) DeactivateAPIKey(keyID uuid.UUID, userID uuid.UUID) error {
	return s.db.Model(&APIKey{}).Where("id = ? AND user_id = ?", keyID, userID).Update("is_active", false).Error
}

// DeleteAPIKey permanently deletes an API key
func (s *APIKeyService) DeleteAPIKey(keyID uuid.UUID, userID uuid.UUID) error {
	return s.db.Where("id = ? AND user_id = ?", keyID, userID).Delete(&APIKey{}).Error
}

// UpdateAPIKeyName updates the name of an API key
func (s *APIKeyService) UpdateAPIKeyName(keyID uuid.UUID, userID uuid.UUID, newName string) error {
	return s.db.Model(&APIKey{}).Where("id = ? AND user_id = ?", keyID, userID).Update("name", newName).Error
}

// CleanupExpiredAPIKeys removes expired API keys
func (s *APIKeyService) CleanupExpiredAPIKeys() error {
	return s.db.Where("expires_at < ?", time.Now()).Delete(&APIKey{}).Error
}

// GetAPIKeyStats returns API key statistics
func (s *APIKeyService) GetAPIKeyStats() (map[string]interface{}, error) {
	var stats map[string]interface{} = make(map[string]interface{})
	
	// Total API keys
	var totalCount int64
	if err := s.db.Model(&APIKey{}).Count(&totalCount).Error; err != nil {
		return nil, err
	}
	stats["total"] = totalCount
	
	// Active API keys
	var activeCount int64
	if err := s.db.Model(&APIKey{}).Where("is_active = ? AND (expires_at IS NULL OR expires_at > ?)", 
		true, time.Now()).Count(&activeCount).Error; err != nil {
		return nil, err
	}
	stats["active"] = activeCount
	
	// Expired API keys
	var expiredCount int64
	if err := s.db.Model(&APIKey{}).Where("expires_at < ?", time.Now()).Count(&expiredCount).Error; err != nil {
		return nil, err
	}
	stats["expired"] = expiredCount
	
	// Recently used API keys (last 24 hours)
	var recentlyUsedCount int64
	if err := s.db.Model(&APIKey{}).Where("last_used > ?", time.Now().Add(-24*time.Hour)).Count(&recentlyUsedCount).Error; err != nil {
		return nil, err
	}
	stats["recently_used"] = recentlyUsedCount
	
	return stats, nil
}

// GetAPIKeyByID retrieves an API key by ID (for the owner)
func (s *APIKeyService) GetAPIKeyByID(keyID uuid.UUID, userID uuid.UUID) (*APIKey, error) {
	var apiKey APIKey
	if err := s.db.Where("id = ? AND user_id = ?", keyID, userID).First(&apiKey).Error; err != nil {
		return nil, err
	}
	return &apiKey, nil
}