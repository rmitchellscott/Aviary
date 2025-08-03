package storage

import (
	"context"
	"fmt"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	internalConfig "github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/logging"
)

// Global storage backend instance
var globalBackend StorageBackendWithInfo
var globalConfig StorageConfig

// InitializeStorage initializes the global storage backend based on configuration
func InitializeStorage() error {
	cfg := GetStorageConfig()
	globalConfig = cfg
	
	var backend StorageBackendWithInfo
	var err error
	
	switch cfg.Backend {
	case "s3":
		backend, err = createS3Backend(cfg)
		if err != nil {
			return fmt.Errorf("failed to create S3 backend: %w", err)
		}
		logging.Logf("[STORAGE] Initialized S3 backend: s3://%s (endpoint: %s)", cfg.S3Bucket, cfg.S3Endpoint)
		
	case "filesystem":
		storageDir := cfg.DataDir
		if !internalConfig.GetBool("MULTI_USER", false) {
			if pdfDir := internalConfig.Get("PDF_DIR", ""); pdfDir != "" {
				storageDir = pdfDir
			}
		}
		backend = NewFilesystemBackend(storageDir)
		logging.Logf("[STORAGE] Initialized filesystem backend: %s", storageDir)
		
	default:
		return fmt.Errorf("unknown storage backend: %s", cfg.Backend)
	}
	
	globalBackend = backend
	return nil
}

// GetStorageBackend returns the initialized global storage backend
func GetStorageBackend() StorageBackendWithInfo {
	if globalBackend == nil {
		// Fallback to filesystem if not initialized
		logging.Logf("[STORAGE] Warning: storage not initialized, falling back to filesystem")
		return NewFilesystemBackend(getDataDir())
	}
	return globalBackend
}

// GetStorageType returns the type of the current storage backend
func GetStorageType() string {
	return globalConfig.Backend
}

// createS3Backend creates and configures an S3 backend
func createS3Backend(cfg StorageConfig) (StorageBackendWithInfo, error) {
	if cfg.S3Bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET is required for S3 backend")
	}
	
	// Create AWS config
	var awsCfg aws.Config
	var err error
	
	// Configure credentials if provided
	if cfg.S3AccessKeyID != "" && cfg.S3SecretKey != "" {
		awsCfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(cfg.S3Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.S3AccessKeyID,
				cfg.S3SecretKey,
				"",
			)),
		)
	} else {
		// Use default credential chain (IAM roles, etc.)
		awsCfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(cfg.S3Region),
		)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}
	
	// Create S3 client with custom endpoint if specified
	var s3Client *s3.Client
	if cfg.S3Endpoint != "" {
		// Parse endpoint to validate
		if _, err := url.Parse(cfg.S3Endpoint); err != nil {
			return nil, fmt.Errorf("invalid S3_ENDPOINT: %w", err)
		}
		
		s3Client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
			o.UsePathStyle = cfg.S3ForcePathStyle
		})
	} else {
		s3Client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.UsePathStyle = cfg.S3ForcePathStyle
		})
	}
	
	// Test S3 connection by listing the bucket
	ctx := context.Background()
	_, err = s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(cfg.S3Bucket),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to access S3 bucket %s: %w", cfg.S3Bucket, err)
	}
	
	backend := NewS3Backend(s3Client, cfg.S3Bucket)
	return backend, nil
}

// ValidateStorageConfig validates the storage configuration
func ValidateStorageConfig(cfg StorageConfig) error {
	switch cfg.Backend {
	case "filesystem":
		// No additional validation needed for filesystem
		return nil
		
	case "s3":
		if cfg.S3Bucket == "" {
			return fmt.Errorf("S3_BUCKET is required for S3 backend")
		}
		if cfg.S3Region == "" {
			return fmt.Errorf("S3_REGION is required for S3 backend")
		}
		// Access key validation is optional (could use IAM roles)
		return nil
		
	default:
		return fmt.Errorf("unknown storage backend: %s (valid options: filesystem, s3)", cfg.Backend)
	}
}