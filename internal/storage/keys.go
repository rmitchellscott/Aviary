package storage

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	internalConfig "github.com/rmitchellscott/aviary/internal/config"
)

// GenerateUserDocumentKey generates a storage key for user documents
func GenerateUserDocumentKey(userID uuid.UUID, prefix, filename string, multiUserMode bool) string {
	if multiUserMode && userID != uuid.Nil {
		if prefix != "" {
			return fmt.Sprintf("users/%s/pdfs/%s/%s", userID.String(), prefix, filename)
		}
		return fmt.Sprintf("users/%s/pdfs/%s", userID.String(), filename)
	}
	
	hasPdfDir := !multiUserMode && internalConfig.Get("PDF_DIR", "") != "" && GetStorageType() == "filesystem"
	if hasPdfDir {
		if prefix != "" {
			return fmt.Sprintf("%s/%s", prefix, filename)
		}
		return filename
	}
	if prefix != "" {
		return fmt.Sprintf("pdfs/%s/%s", prefix, filename)
	}
	return fmt.Sprintf("pdfs/%s", filename)
}

// GenerateUserConfigKey generates a storage key for user rmapi config
func GenerateUserConfigKey(userID uuid.UUID) string {
	return fmt.Sprintf("users/%s/rmapi/rmapi.conf", userID.String())
}

// GenerateBackupKey generates a storage key for backup files
func GenerateBackupKey(filename string) string {
	return fmt.Sprintf("backups/%s", filename)
}

// GenerateUserPrefix returns the storage prefix for a user's files
func GenerateUserPrefix(userID uuid.UUID) string {
	return fmt.Sprintf("users/%s/", userID.String())
}

// GenerateUserDocumentPrefix returns the storage prefix for a user's documents
func GenerateUserDocumentPrefix(userID uuid.UUID) string {
	return fmt.Sprintf("users/%s/pdfs/", userID.String())
}

// GenerateUserConfigPrefix returns the storage prefix for a user's configs
func GenerateUserConfigPrefix(userID uuid.UUID) string {
	return fmt.Sprintf("users/%s/rmapi/", userID.String())
}

// ParseUserIDFromKey extracts the user ID from a storage key, if present
func ParseUserIDFromKey(storageKey string) (uuid.UUID, error) {
	if !strings.HasPrefix(storageKey, "users/") {
		return uuid.Nil, fmt.Errorf("storage key is not user-specific: %s", storageKey)
	}
	
	parts := strings.Split(storageKey, "/")
	if len(parts) < 2 {
		return uuid.Nil, fmt.Errorf("invalid user storage key format: %s", storageKey)
	}
	
	userID, err := uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid user ID in storage key: %w", err)
	}
	
	return userID, nil
}

// IsUserStorageKey checks if a storage key belongs to a specific user
func IsUserStorageKey(storageKey string, userID uuid.UUID) bool {
	expectedPrefix := GenerateUserPrefix(userID)
	return strings.HasPrefix(storageKey, expectedPrefix)
}

// IsBackupStorageKey checks if a storage key is for a backup file
func IsBackupStorageKey(storageKey string) bool {
	return strings.HasPrefix(storageKey, "backups/")
}

// SanitizeStorageKey ensures the storage key is safe to use
func SanitizeStorageKey(key string) string {
	// Remove any leading slashes
	key = strings.TrimPrefix(key, "/")
	
	// Replace backslashes with forward slashes
	key = strings.ReplaceAll(key, "\\", "/")
	
	// Remove any double slashes
	for strings.Contains(key, "//") {
		key = strings.ReplaceAll(key, "//", "/")
	}
	
	return key
}