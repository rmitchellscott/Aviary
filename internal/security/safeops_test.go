package security

import (
	"testing"
	"os"
	"path/filepath"
)

func TestSafeOperations(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("SafeCreate and SafeRemove", func(t *testing.T) {
		testPath := filepath.Join(tempDir, "test.txt")
		sp, err := NewSecurePathFromExisting(testPath)
		if err != nil {
			t.Fatalf("Failed to create SecurePath: %v", err)
		}

		file, err := SafeCreate(sp)
		if err != nil {
			t.Errorf("SafeCreate() error = %v", err)
		}
		if file != nil {
			file.Close()
		}

		if !SafeStatExists(sp) {
			t.Error("File should exist after SafeCreate")
		}

		err = SafeRemove(sp)
		if err != nil {
			t.Errorf("SafeRemove() error = %v", err)
		}

		if SafeStatExists(sp) {
			t.Error("File should not exist after SafeRemove")
		}
	})

	t.Run("SafeOpen", func(t *testing.T) {
		testPath := filepath.Join(tempDir, "testread.txt")
		err := os.WriteFile(testPath, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		sp, err := NewSecurePathFromExisting(testPath)
		if err != nil {
			t.Fatalf("Failed to create SecurePath: %v", err)
		}

		file, err := SafeOpen(sp)
		if err != nil {
			t.Errorf("SafeOpen() error = %v", err)
		}
		if file != nil {
			file.Close()
		}
	})

	t.Run("SafeMkdirAll", func(t *testing.T) {
		testDir := filepath.Join(tempDir, "newdir", "subdir")
		sp, err := NewSecurePathFromExisting(testDir)
		if err != nil {
			t.Fatalf("Failed to create SecurePath: %v", err)
		}

		err = SafeMkdirAll(sp, 0755)
		if err != nil {
			t.Errorf("SafeMkdirAll() error = %v", err)
		}

		if !SafeStatExists(sp) {
			t.Error("Directory should exist after SafeMkdirAll")
		}
	})

	t.Run("SafeStat", func(t *testing.T) {
		testPath := filepath.Join(tempDir, "stattest.txt")
		err := os.WriteFile(testPath, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		sp, err := NewSecurePathFromExisting(testPath)
		if err != nil {
			t.Fatalf("Failed to create SecurePath: %v", err)
		}

		info, err := SafeStat(sp)
		if err != nil {
			t.Errorf("SafeStat() error = %v", err)
		}
		if info == nil {
			t.Error("SafeStat() returned nil info")
		}
		if info != nil && info.Size() != 4 {
			t.Errorf("SafeStat() size = %d, want 4", info.Size())
		}
	})

	t.Run("SafeRename", func(t *testing.T) {
		oldPath := filepath.Join(tempDir, "old.txt")
		newPath := filepath.Join(tempDir, "new.txt")
		
		err := os.WriteFile(oldPath, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		oldSP, err := NewSecurePathFromExisting(oldPath)
		if err != nil {
			t.Fatalf("Failed to create old SecurePath: %v", err)
		}
		
		newSP, err := NewSecurePathFromExisting(newPath)
		if err != nil {
			t.Fatalf("Failed to create new SecurePath: %v", err)
		}

		err = SafeRename(oldSP, newSP)
		if err != nil {
			t.Errorf("SafeRename() error = %v", err)
		}

		if SafeStatExists(oldSP) {
			t.Error("Old file should not exist after rename")
		}
		if !SafeStatExists(newSP) {
			t.Error("New file should exist after rename")
		}
	})

	t.Run("SafeRemoveIfExists", func(t *testing.T) {
		testPath := filepath.Join(tempDir, "remove_test.txt")
		sp, err := NewSecurePathFromExisting(testPath)
		if err != nil {
			t.Fatalf("Failed to create SecurePath: %v", err)
		}

		// Test removing non-existent file (should not error)
		err = SafeRemoveIfExists(sp)
		if err != nil {
			t.Errorf("SafeRemoveIfExists() should not error for non-existent file: %v", err)
		}

		// Create file and remove it
		file, err := SafeCreate(sp)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		file.Close()

		err = SafeRemoveIfExists(sp)
		if err != nil {
			t.Errorf("SafeRemoveIfExists() error = %v", err)
		}

		if SafeStatExists(sp) {
			t.Error("File should not exist after SafeRemoveIfExists")
		}
	})

	t.Run("nil SecurePath operations", func(t *testing.T) {
		var sp *SecurePath

		_, err := SafeCreate(sp)
		if err == nil {
			t.Error("SafeCreate(nil) should return error")
		}

		err = SafeRemove(sp)
		if err == nil {
			t.Error("SafeRemove(nil) should return error")
		}

		err = SafeMkdirAll(sp, 0755)
		if err == nil {
			t.Error("SafeMkdirAll(nil) should return error")
		}

		_, err = SafeOpen(sp)
		if err == nil {
			t.Error("SafeOpen(nil) should return error")
		}

		_, err = SafeStat(sp)
		if err == nil {
			t.Error("SafeStat(nil) should return error")
		}

		if SafeStatExists(sp) {
			t.Error("SafeStatExists(nil) should return false")
		}

		err = SafeRemoveIfExists(sp)
		if err != nil {
			t.Error("SafeRemoveIfExists(nil) should not return error")
		}
	})
}