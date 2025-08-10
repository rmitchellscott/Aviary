package security

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var (
	ErrPathTraversal     = errors.New("path contains directory traversal sequences")
	ErrAbsolutePath      = errors.New("absolute paths are not allowed")
	ErrEmptyPath         = errors.New("path cannot be empty")
	ErrInvalidPath       = errors.New("invalid path")
	ErrOutsideBaseDir    = errors.New("path is outside allowed base directory")
)

func ValidateFilePath(path string) error {
	if path == "" {
		return ErrEmptyPath
	}

	if strings.Contains(path, "..") {
		return ErrPathTraversal
	}

	if filepath.IsAbs(path) {
		return ErrAbsolutePath
	}

	cleanPath := filepath.Clean(path)
	if cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return ErrPathTraversal
	}

	if strings.Contains(path, "\x00") {
		return ErrInvalidPath
	}

	return nil
}

func ValidateFilePathInBase(path, baseDir string) error {
	if err := ValidateFilePath(path); err != nil {
		return err
	}

	if baseDir == "" {
		return fmt.Errorf("base directory cannot be empty")
	}

	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("failed to resolve base directory: %w", err)
	}

	fullPath := filepath.Join(absBaseDir, path)
	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return fmt.Errorf("failed to resolve full path: %w", err)
	}

	if !strings.HasPrefix(absFullPath, absBaseDir+string(filepath.Separator)) {
		if absFullPath != absBaseDir {
			return ErrOutsideBaseDir
		}
	}

	return nil
}

func ValidateStorageKey(key string) error {
	if key == "" {
		return ErrEmptyPath
	}

	if strings.Contains(key, "..") {
		return ErrPathTraversal
	}

	if filepath.IsAbs(key) {
		return ErrAbsolutePath
	}

	cleanKey := filepath.Clean(key)
	if cleanKey == "." || cleanKey == ".." || strings.HasPrefix(cleanKey, "../") {
		return ErrPathTraversal
	}

	if strings.Contains(key, "\x00") {
		return ErrInvalidPath
	}

	parts := strings.Split(key, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return ErrPathTraversal
		}
	}

	return nil
}

func ValidateAndCleanFilename(filename string) (string, error) {
	if filename == "" {
		return "", ErrEmptyPath
	}

	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return "", fmt.Errorf("filename cannot contain path separators")
	}

	if strings.Contains(filename, "..") {
		return "", ErrPathTraversal
	}

	if strings.Contains(filename, "\x00") {
		return "", ErrInvalidPath
	}

	filename = strings.TrimSpace(filename)
	if filename == "" {
		return "", ErrEmptyPath
	}

	return filename, nil
}

func SafeJoin(basePath, userPath string) (string, error) {
	if err := ValidateFilePathInBase(userPath, basePath); err != nil {
		return "", err
	}

	result := filepath.Join(basePath, userPath)
	return result, nil
}

func ValidateExistingFilePath(path string) error {
	if path == "" {
		return ErrEmptyPath
	}

	if strings.Contains(path, "\x00") {
		return ErrInvalidPath
	}

	if !filepath.IsAbs(path) {
		return ErrAbsolutePath
	}

	if strings.Contains(path, "..") {
		return ErrPathTraversal
	}

	cleanPath := filepath.Clean(path)
	
	if strings.Contains(cleanPath, "..") {
		return ErrPathTraversal
	}

	return nil
}