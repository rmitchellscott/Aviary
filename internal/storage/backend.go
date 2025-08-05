package storage

import (
	"context"
	"io"
)

// StorageBackend defines the interface for different storage implementations
type StorageBackend interface {
	// Put stores data at the given key
	Put(ctx context.Context, key string, data io.Reader) error
	
	// Get retrieves data from the given key
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	
	// Delete removes the object at the given key
	Delete(ctx context.Context, key string) error
	
	// List returns all keys with the given prefix
	List(ctx context.Context, prefix string) ([]string, error)
	
	// Exists checks if an object exists at the given key
	Exists(ctx context.Context, key string) (bool, error)
	
	// Copy copies an object from srcKey to dstKey
	Copy(ctx context.Context, srcKey, dstKey string) error
}

// StorageInfo provides metadata about stored objects
type StorageInfo struct {
	Key          string
	Size         int64
	LastModified string
}

// StorageBackendWithInfo extends StorageBackend with metadata operations
type StorageBackendWithInfo interface {
	StorageBackend
	
	// ListWithInfo returns objects with metadata
	ListWithInfo(ctx context.Context, prefix string) ([]StorageInfo, error)
	
	// GetInfo returns metadata for a single object
	GetInfo(ctx context.Context, key string) (*StorageInfo, error)
}