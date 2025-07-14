package manager

import (
	"os"
	"path/filepath"
	
	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/database"
)

// GetUserDataDir returns the data directory for a specific user
// In multi-user mode, creates /data/users/{user_id}/
// In single-user mode, returns /data/
func GetUserDataDir(userID uuid.UUID) (string, error) {
	baseDir := os.Getenv("DATA_DIR")
	if baseDir == "" {
		baseDir = "/data"
	}
	
	if database.IsMultiUserMode() {
		userDir := filepath.Join(baseDir, "users", userID.String())
		if err := os.MkdirAll(userDir, 0755); err != nil {
			return "", err
		}
		return userDir, nil
	}
	
	// Single-user mode - use base directory
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", err
	}
	return baseDir, nil
}

// GetUserPDFDir returns the PDF directory for a specific user
// In multi-user mode, creates /data/users/{user_id}/pdfs/
// In single-user mode, returns the PDF_DIR env var or /data/pdfs/
func GetUserPDFDir(userID uuid.UUID, prefix string) (string, error) {
	var baseDir string
	
	if database.IsMultiUserMode() {
		userDataDir, err := GetUserDataDir(userID)
		if err != nil {
			return "", err
		}
		baseDir = filepath.Join(userDataDir, "pdfs")
	} else {
		// Single-user mode - use PDF_DIR environment variable
		baseDir = os.Getenv("PDF_DIR")
		if baseDir == "" {
			baseDir = "/data/pdfs"
		}
	}
	
	// Add prefix subdirectory if provided
	if prefix != "" {
		baseDir = filepath.Join(baseDir, prefix)
	}
	
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", err
	}
	return baseDir, nil
}

// GetUserUploadDir returns the upload directory for a specific user
// In multi-user mode, creates /data/users/{user_id}/uploads/
// In single-user mode, returns ./uploads/
func GetUserUploadDir(userID uuid.UUID) (string, error) {
	if database.IsMultiUserMode() {
		userDataDir, err := GetUserDataDir(userID)
		if err != nil {
			return "", err
		}
		uploadDir := filepath.Join(userDataDir, "uploads")
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			return "", err
		}
		return uploadDir, nil
	}
	
	// Single-user mode - use ./uploads
	uploadDir := "./uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", err
	}
	return uploadDir, nil
}

// GetUserTempDir returns the temp directory for a specific user
// In multi-user mode, creates /data/users/{user_id}/temp/
// In single-user mode, returns system temp directory
func GetUserTempDir(userID uuid.UUID) (string, error) {
	if database.IsMultiUserMode() {
		userDataDir, err := GetUserDataDir(userID)
		if err != nil {
			return "", err
		}
		tempDir := filepath.Join(userDataDir, "temp")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			return "", err
		}
		return tempDir, nil
	}
	
	// Single-user mode - use system temp directory
	return os.TempDir(), nil
}

// CleanupUserTempDir removes old temp files for a user
func CleanupUserTempDir(userID uuid.UUID) error {
	if !database.IsMultiUserMode() {
		return nil // Don't cleanup system temp directory
	}
	
	userDataDir, err := GetUserDataDir(userID)
	if err != nil {
		return err
	}
	
	tempDir := filepath.Join(userDataDir, "temp")
	if err := os.RemoveAll(tempDir); err != nil {
		return err
	}
	
	return os.MkdirAll(tempDir, 0755)
}