package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/rmitchellscott/aviary/internal/logging"
	"github.com/rmitchellscott/aviary/internal/security"
)

func StreamToResponse(ctx context.Context, c *gin.Context, storageKey, filename, contentType string) error {
	backend := GetStorageBackend()
	exists, err := backend.Exists(ctx, storageKey)
	if err != nil {
		return fmt.Errorf("failed to check file existence: %w", err)
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return fmt.Errorf("file not found: %s", storageKey)
	}
	
	reader, err := backend.Get(ctx, storageKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file"})
		return fmt.Errorf("failed to get file from storage: %w", err)
	}
	defer reader.Close()
	
	if filename != "" {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	}
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}
	
	if _, err := io.Copy(c.Writer, reader); err != nil {
		logging.Logf("[ERROR] Failed to stream file %s: %v", storageKey, err)
		return fmt.Errorf("failed to stream file: %w", err)
	}
	
	return nil
}

func CopyFileToStorage(ctx context.Context, sourcePath, storageKey string) error {
	secureSourcePath, err := security.NewSecurePathFromExisting(sourcePath)
	if err != nil {
		return fmt.Errorf("invalid source path %s: %w", sourcePath, err)
	}
	backend := GetStorageBackend()
	sourceFile, err := security.SafeOpen(secureSourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", sourcePath, err)
	}
	defer sourceFile.Close()
	
	if err := backend.Put(ctx, storageKey, sourceFile); err != nil {
		return fmt.Errorf("failed to store file %s: %w", storageKey, err)
	}
	
	return nil
}

func CopyFileFromStorage(ctx context.Context, storageKey, destPath string) error {
	backend := GetStorageBackend()
	
	reader, err := backend.Get(ctx, storageKey)
	if err != nil {
		return fmt.Errorf("failed to get file from storage %s: %w", storageKey, err)
	}
	defer reader.Close()
	
	secureDestPath, err := security.NewSecurePathFromExisting(destPath)
	if err != nil {
		return fmt.Errorf("invalid destination path %s: %w", destPath, err)
	}
	
	dirPath, err := security.NewSecurePathFromExisting(filepath.Dir(destPath))
	if err != nil {
		return fmt.Errorf("invalid directory path %s: %w", filepath.Dir(destPath), err)
	}
	
	if err := security.SafeMkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	
	destFile, err := security.SafeCreate(secureDestPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
	}
	defer destFile.Close()
	
	if _, err := io.Copy(destFile, reader); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}
	
	return nil
}

func CleanupStorageByPrefix(ctx context.Context, prefix string) error {
	backend := GetStorageBackend()
	keys, err := backend.List(ctx, prefix)
	if err != nil {
		return fmt.Errorf("failed to list files with prefix %s: %w", prefix, err)
	}
	
	for _, key := range keys {
		if err := backend.Delete(ctx, key); err != nil {
			logging.Logf("[WARNING] Failed to delete storage file %s: %v", key, err)
		}
	}
	
	if len(keys) > 0 {
		logging.Logf("[STORAGE] Cleaned up %d files with prefix %s", len(keys), prefix)
	}
	
	return nil
}

func MoveFileToStorage(ctx context.Context, sourcePath, storageKey string) error {
	if err := CopyFileToStorage(ctx, sourcePath, storageKey); err != nil {
		return err
	}
	
	secureSourcePath, err := security.NewSecurePathFromExisting(sourcePath)
	if err != nil {
		return fmt.Errorf("invalid source path %s: %w", sourcePath, err)
	}
	
	if err := security.SafeRemove(secureSourcePath); err != nil {
		logging.Logf("[WARNING] Failed to remove source file after move: %v", err)
	}
	
	return nil
}

func EnsureStorageKeyExists(ctx context.Context, storageKey string) error {
	backend := GetStorageBackend()
	
	exists, err := backend.Exists(ctx, storageKey)
	if err != nil {
		return fmt.Errorf("failed to check if storage key exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("storage key does not exist: %s", storageKey)
	}
	
	return nil
}

func MoveInStorage(ctx context.Context, srcKey, dstKey string) error {
	backend := GetStorageBackend()
	if err := backend.Copy(ctx, srcKey, dstKey); err != nil {
		return fmt.Errorf("failed to copy file from %s to %s: %w", srcKey, dstKey, err)
	}
	
	if err := backend.Delete(ctx, srcKey); err != nil {
		logging.Logf("[WARNING] Failed to delete source file after move: %v", err)
	}
	
	return nil
}