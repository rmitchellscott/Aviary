package downloads

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/logging"
	"github.com/rmitchellscott/aviary/internal/storage"
)

type DownloadEntry struct {
	Token      string
	StorageKey string
	Filename   string
	ExpiresAt  time.Time
}

var (
	mu       sync.RWMutex
	entries  = make(map[string]*DownloadEntry)
)

func ttl() time.Duration {
	return config.GetDuration("DOWNLOAD_LINK_TTL", 1*time.Hour)
}

func Register(localPath, filename string) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	storageKey := fmt.Sprintf("downloads/%s/%s", token, filepath.Base(filename))

	ctx := context.Background()
	if err := storage.CopyFileToStorage(ctx, localPath, storageKey); err != nil {
		return "", fmt.Errorf("failed to persist file for download: %w", err)
	}

	entry := &DownloadEntry{
		Token:      token,
		StorageKey: storageKey,
		Filename:   filepath.Base(filename),
		ExpiresAt:  time.Now().Add(ttl()),
	}

	mu.Lock()
	entries[token] = entry
	mu.Unlock()

	logging.Logf("[DOWNLOADS] Registered download token %s for %s (expires %s)", token[:8]+"...", filename, entry.ExpiresAt.Format(time.RFC3339))
	return token, nil
}

func Get(token string) (*DownloadEntry, error) {
	mu.RLock()
	entry, ok := entries[token]
	mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("download not found")
	}
	if time.Now().After(entry.ExpiresAt) {
		go removeEntry(token)
		return nil, fmt.Errorf("download expired")
	}
	return entry, nil
}

func removeEntry(token string) {
	mu.Lock()
	entry, ok := entries[token]
	delete(entries, token)
	mu.Unlock()

	if ok {
		ctx := context.Background()
		backend := storage.GetStorageBackend()
		if err := backend.Delete(ctx, entry.StorageKey); err != nil {
			logging.Logf("[DOWNLOADS] Warning: failed to delete expired file %s: %v", entry.StorageKey, err)
		}
	}
}

func StartCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			mu.RLock()
			var expired []string
			for token, entry := range entries {
				if now.After(entry.ExpiresAt) {
					expired = append(expired, token)
				}
			}
			mu.RUnlock()

			for _, token := range expired {
				removeEntry(token)
			}
			if len(expired) > 0 {
				logging.Logf("[DOWNLOADS] Cleaned up %d expired downloads", len(expired))
			}
		}
	}()
}

func CleanupAll(ctx context.Context) {
	if err := storage.CleanupStorageByPrefix(ctx, "downloads/"); err != nil {
		logging.Logf("[DOWNLOADS] Warning: startup cleanup failed: %v", err)
	}
}
