package security

import (
	"testing"
	"os"
	"path/filepath"
)

// TestSecurityIntegration tests that our security measures work correctly
// in scenarios similar to those that would trigger CodeQL alerts
func TestSecurityIntegration(t *testing.T) {
	// Test path traversal attempts that CodeQL would flag as vulnerabilities
	maliciousPaths := []string{
		"../etc/passwd",
		"../../etc/passwd",
		"../../../etc/passwd",
		"/etc/passwd",
		"dir/../../../etc/passwd", 
		"file\x00.txt",
		"..",
		".",
		"../",
	}
	
	// Additional paths that should be rejected by storage key validation
	storageKeyMaliciousPaths := []string{
		"../etc/passwd",
		"../../etc/passwd",
		"../../../etc/passwd",
		"/etc/passwd",
		"dir/../../../etc/passwd", 
		"file\x00.txt",
		"..",
		".",
		"../",
		"dir//file.txt",
		"dir/./file.txt",
	}

	t.Run("ValidateFilePath blocks malicious paths", func(t *testing.T) {
		for _, path := range maliciousPaths {
			err := ValidateFilePath(path)
			if err == nil {
				t.Errorf("ValidateFilePath should reject malicious path: %s", path)
			}
		}
	})

	t.Run("ValidateStorageKey blocks malicious keys", func(t *testing.T) {
		for _, key := range storageKeyMaliciousPaths {
			err := ValidateStorageKey(key)
			if err == nil {
				t.Errorf("ValidateStorageKey should reject malicious key: %s", key)
			}
		}
	})

	t.Run("SafeJoin prevents path traversal", func(t *testing.T) {
		baseDir := "/tmp/safe"
		for _, path := range maliciousPaths {
			_, err := SafeJoin(baseDir, path)
			if err == nil {
				t.Errorf("SafeJoin should reject malicious path: %s", path)
			}
		}
	})

	// Test that legitimate paths are still allowed
	legitimatePaths := []string{
		"file.txt",
		"dir/file.txt", 
		"a/b/c/file.pdf",
		"document.pdf",
		"image.jpg",
	}

	t.Run("ValidateFilePath allows legitimate paths", func(t *testing.T) {
		for _, path := range legitimatePaths {
			err := ValidateFilePath(path)
			if err != nil {
				t.Errorf("ValidateFilePath should allow legitimate path %s, but got error: %v", path, err)
			}
		}
	})
}

// TestFileOperationSafety simulates the file operations that were vulnerable to CodeQL alerts
func TestFileOperationSafety(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "security-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test safe file creation (like the fixed os.Create operations)
	t.Run("Safe file operations work correctly", func(t *testing.T) {
		validFilename := "test.txt"
		
		// Validate first (like our fixed code does)
		if err := ValidateFilePath(validFilename); err != nil {
			t.Fatalf("Valid filename rejected: %v", err)
		}
		
		// Then perform the file operation
		safePath := filepath.Join(tempDir, validFilename)
		file, err := os.Create(safePath)
		if err != nil {
			t.Fatalf("Failed to create safe file: %v", err)
		}
		file.Close()
		
		// Verify file exists
		if _, err := os.Stat(safePath); err != nil {
			t.Errorf("Safe file was not created: %v", err)
		}
	})

	t.Run("Malicious filenames are rejected", func(t *testing.T) {
		maliciousFilename := "../../../etc/passwd"
		
		// Our validation should reject this
		if err := ValidateFilePath(maliciousFilename); err == nil {
			t.Fatalf("Malicious filename was not rejected")
		}
		
		// Our validation prevents malicious paths from reaching file operations
		// This demonstrates the security fix - the malicious path is blocked before any file operations
	})
}