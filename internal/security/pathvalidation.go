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
	for i, part := range parts {
		if part == "." || part == ".." {
			return ErrPathTraversal
		}
		// Allow empty parts only if it's the last component (trailing slash for prefixes)
		if part == "" && i != len(parts)-1 {
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

type SecurePath struct {
	path string
}

func NewSecurePath(userPath string) (*SecurePath, error) {
	if err := ValidateFilePath(userPath); err != nil {
		return nil, err
	}
	cleanPath := filepath.Clean(userPath)
	return &SecurePath{path: cleanPath}, nil
}

func NewSecurePathFromExisting(existingPath string) (*SecurePath, error) {
	if err := ValidateExistingFilePath(existingPath); err != nil {
		return nil, err
	}
	cleanPath := filepath.Clean(existingPath)
	return &SecurePath{path: cleanPath}, nil
}

func NewSecurePathInBase(userPath, baseDir string) (*SecurePath, error) {
	if err := ValidateFilePathInBase(userPath, baseDir); err != nil {
		return nil, err
	}
	fullPath := filepath.Join(baseDir, userPath)
	cleanPath := filepath.Clean(fullPath)
	return &SecurePath{path: cleanPath}, nil
}

func NewSecureStorageKey(key string) (*SecurePath, error) {
	if err := ValidateStorageKey(key); err != nil {
		return nil, err
	}
	return &SecurePath{path: key}, nil
}

func (sp *SecurePath) String() string {
	if sp == nil {
		return ""
	}
	return sp.path
}

func (sp *SecurePath) Dir() *SecurePath {
	if sp == nil {
		return nil
	}
	return &SecurePath{path: filepath.Dir(sp.path)}
}

func (sp *SecurePath) Join(elem string) (*SecurePath, error) {
	if sp == nil {
		return nil, fmt.Errorf("cannot join to nil SecurePath")
	}
	cleanElem := filepath.Clean(elem)
	if strings.Contains(cleanElem, "..") {
		return nil, ErrPathTraversal
	}
	joined := filepath.Join(sp.path, cleanElem)
	return &SecurePath{path: joined}, nil
}