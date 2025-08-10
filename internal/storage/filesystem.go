package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rmitchellscott/aviary/internal/logging"
	"github.com/rmitchellscott/aviary/internal/security"
)

type FilesystemBackend struct {
	basePath string
}

func NewFilesystemBackend(basePath string) *FilesystemBackend {
	return &FilesystemBackend{
		basePath: basePath,
	}
}

func (fs *FilesystemBackend) Put(ctx context.Context, key string, data io.Reader) error {
	filePath, err := fs.keyToPath(key)
	if err != nil {
		return fmt.Errorf("invalid storage key %s: %w", key, err)
	}
	dirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		logging.Logf("[STORAGE] ERROR: Failed to create directory %s: %v", dirPath, err)
		return fmt.Errorf("failed to create directory for %s: %w", key, err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		logging.Logf("[STORAGE] ERROR: Failed to create file %s: %v", filePath, err)
		return fmt.Errorf("failed to create file %s: %w", key, err)
	}
	defer file.Close()

	_, err = io.Copy(file, data)
	if err != nil {
		logging.Logf("[STORAGE] ERROR: Failed to write data to %s: %v", filePath, err)
		return fmt.Errorf("failed to write data to %s: %w", key, err)
	}

	return nil
}

func (fs *FilesystemBackend) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	filePath, err := fs.keyToPath(key)
	if err != nil {
		return nil, fmt.Errorf("invalid storage key %s: %w", key, err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return nil, fmt.Errorf("failed to open file %s: %w", key, err)
	}

	return file, nil
}

func (fs *FilesystemBackend) Delete(ctx context.Context, key string) error {
	filePath, err := fs.keyToPath(key)
	if err != nil {
		return fmt.Errorf("invalid storage key %s: %w", key, err)
	}

	err = os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete %s: %w", key, err)
	}

	// logging.Logf("[STORAGE] Delete: %s", key)
	return nil
}

func (fs *FilesystemBackend) List(ctx context.Context, prefix string) ([]string, error) {
	prefixPath, err := fs.keyToPath(prefix)
	if err != nil {
		return nil, fmt.Errorf("invalid storage prefix %s: %w", prefix, err)
	}
	var keys []string
	if _, err := os.Stat(prefixPath); os.IsNotExist(err) {
		return keys, nil
	}

	err = filepath.Walk(prefixPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		key := fs.pathToKey(path)
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list keys with prefix %s: %w", prefix, err)
	}

	return keys, nil
}

func (fs *FilesystemBackend) Exists(ctx context.Context, key string) (bool, error) {
	filePath, err := fs.keyToPath(key)
	if err != nil {
		return false, fmt.Errorf("invalid storage key %s: %w", key, err)
	}

	_, err = os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check existence of %s: %w", key, err)
	}

	return true, nil
}

func (fs *FilesystemBackend) Copy(ctx context.Context, srcKey, dstKey string) error {
	srcPath, err := fs.keyToPath(srcKey)
	if err != nil {
		return fmt.Errorf("invalid source storage key %s: %w", srcKey, err)
	}
	dstPath, err := fs.keyToPath(dstKey)
	if err != nil {
		return fmt.Errorf("invalid destination storage key %s: %w", dstKey, err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", dstKey, err)
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source %s: %w", srcKey, err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination %s: %w", dstKey, err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy data from %s to %s: %w", srcKey, dstKey, err)
	}

	logging.Logf("[STORAGE] Copy: %s -> %s", srcKey, dstKey)
	return nil
}

func (fs *FilesystemBackend) ListWithInfo(ctx context.Context, prefix string) ([]StorageInfo, error) {
	prefixPath, err := fs.keyToPath(prefix)
	if err != nil {
		return nil, fmt.Errorf("invalid storage prefix %s: %w", prefix, err)
	}
	var infos []StorageInfo
	if _, err := os.Stat(prefixPath); os.IsNotExist(err) {
		return infos, nil
	}

	err = filepath.Walk(prefixPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		key := fs.pathToKey(path)
		if strings.HasPrefix(key, prefix) {
			infos = append(infos, StorageInfo{
				Key:          key,
				Size:         info.Size(),
				LastModified: info.ModTime().Format(time.RFC3339),
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list objects with prefix %s: %w", prefix, err)
	}

	return infos, nil
}

func (fs *FilesystemBackend) GetInfo(ctx context.Context, key string) (*StorageInfo, error) {
	filePath, err := fs.keyToPath(key)
	if err != nil {
		return nil, fmt.Errorf("invalid storage key %s: %w", key, err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return nil, fmt.Errorf("failed to get info for %s: %w", key, err)
	}

	return &StorageInfo{
		Key:          key,
		Size:         info.Size(),
		LastModified: info.ModTime().Format(time.RFC3339),
	}, nil
}

func (fs *FilesystemBackend) keyToPath(key string) (string, error) {
	if err := security.ValidateStorageKey(key); err != nil {
		return "", err
	}
	return filepath.Join(fs.basePath, filepath.FromSlash(key)), nil
}

func (fs *FilesystemBackend) pathToKey(path string) string {
	relPath, err := filepath.Rel(fs.basePath, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(relPath)
}
