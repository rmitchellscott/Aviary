package storage

import (
	internalConfig "github.com/rmitchellscott/aviary/internal/config"
)

// StorageConfig holds configuration for storage backends
type StorageConfig struct {
	Backend          string
	DataDir          string
	S3Endpoint       string
	S3Region         string
	S3Bucket         string
	S3AccessKeyID    string
	S3SecretKey      string
	S3ForcePathStyle bool
}

// GetStorageConfig returns storage configuration from environment variables
func GetStorageConfig() StorageConfig {
	return StorageConfig{
		Backend:          internalConfig.Get("STORAGE_BACKEND", "filesystem"),
		DataDir:          getDataDir(),
		S3Endpoint:       internalConfig.Get("S3_ENDPOINT", ""),
		S3Region:         internalConfig.Get("S3_REGION", "us-east-1"),
		S3Bucket:         internalConfig.Get("S3_BUCKET", ""),
		S3AccessKeyID:    internalConfig.Get("S3_ACCESS_KEY_ID", ""),
		S3SecretKey:      internalConfig.Get("S3_SECRET_ACCESS_KEY", ""),
		S3ForcePathStyle: internalConfig.GetBool("S3_FORCE_PATH_STYLE", false),
	}
}

func getDataDir() string {
	if dir := internalConfig.Get("DATA_DIR", ""); dir != "" {
		return dir
	}
	return "/data"
}

func getPdfDir() string {
	isMultiUser := internalConfig.GetBool("MULTI_USER", false)
	
	if !isMultiUser {
		if pdfDir := internalConfig.Get("PDF_DIR", ""); pdfDir != "" {
			return pdfDir
		}
	}
	
	return getDataDir()
}

