package manager

import (
	"io/ioutil"

	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/database"
)

// CreateUserTempDir creates a temporary directory for user-specific operations
// Uses Go's native temp directory handling with user isolation
func CreateUserTempDir(userID uuid.UUID) (string, error) {
	if database.IsMultiUserMode() && userID != uuid.Nil {
		prefix := "aviary-user-" + userID.String() + "-"
		return ioutil.TempDir("", prefix)
	}
	// Single-user mode
	return ioutil.TempDir("", "aviary-")
}

// CreateUserTempFile creates a temporary file for user-specific operations
// Uses Go's native temp file handling with user isolation
func CreateUserTempFile(userID uuid.UUID, pattern string) (string, error) {
	tempDir, err := CreateUserTempDir(userID)
	if err != nil {
		return "", err
	}
	
	if pattern == "" {
		pattern = "file-*"
	}
	
	tempFile, err := ioutil.TempFile(tempDir, pattern)
	if err != nil {
		return "", err
	}
	
	path := tempFile.Name()
	tempFile.Close()
	return path, nil
}

