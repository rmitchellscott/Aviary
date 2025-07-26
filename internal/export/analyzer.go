package export

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
)

// BackupAnalysis contains the results of analyzing a backup file
type BackupAnalysis struct {
	Valid           bool                   `json:"valid"`
	AviaryVersion   string                 `json:"aviary_version"`
	GitCommit       string                 `json:"git_commit"`
	ExportTimestamp string                 `json:"export_timestamp"`
	DatabaseType    string                 `json:"database_type"`
	UserCount       int                    `json:"user_count"`
	APIKeyCount     int                    `json:"api_key_count"`
	DocumentCount   int64                  `json:"document_count"`
	TotalSizeBytes  int64                  `json:"total_size_bytes"`
	ExportedTables  []string               `json:"exported_tables"`
	UsersExported   []string               `json:"users_exported"`
	Errors          []string               `json:"errors,omitempty"`
	Warnings        []string               `json:"warnings,omitempty"`
}

// Analyzer handles analyzing backup files
type Analyzer struct {
	db      *gorm.DB
	dataDir string
}

// NewAnalyzer creates a new analyzer instance
func NewAnalyzer(db *gorm.DB, dataDir string) *Analyzer {
	return &Analyzer{
		db:      db,
		dataDir: dataDir,
	}
}

// AnalyzeBackup analyzes a backup file and returns metadata without restoring
func (a *Analyzer) AnalyzeBackup(archivePath string) (*BackupAnalysis, error) {
	// Create temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "aviary-analyze-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract only metadata and database files for analysis
	if err := a.extractForAnalysis(archivePath, tempDir); err != nil {
		return nil, fmt.Errorf("failed to extract archive: %w", err)
	}

	// Read metadata
	metadataPath := filepath.Join(tempDir, "metadata.json")
	var metadata ExportMetadata
	if err := readJSON(metadataPath, &metadata); err != nil {
		return &BackupAnalysis{
			Valid:  false,
			Errors: []string{"Failed to read metadata.json: " + err.Error()},
		}, nil
	}

	analysis := &BackupAnalysis{
		Valid:           true,
		AviaryVersion:   metadata.AviaryVersion,
		GitCommit:       metadata.GitCommit,
		ExportTimestamp: metadata.ExportTimestamp.Format("2006-01-02 15:04:05 UTC"),
		DatabaseType:    metadata.DatabaseType,
		DocumentCount:   metadata.TotalDocuments,
		TotalSizeBytes:  metadata.TotalSizeBytes,
		ExportedTables:  metadata.ExportedTables,
		UsersExported:   metadata.UsersExported,
	}

	// Analyze database files
	dbDir := filepath.Join(tempDir, "database")
	if stat, err := os.Stat(dbDir); err == nil && stat.IsDir() {
		if err := a.analyzeDatabaseFiles(dbDir, analysis); err != nil {
			analysis.Warnings = append(analysis.Warnings, "Failed to analyze database files: "+err.Error())
		}
	} else {
		analysis.Warnings = append(analysis.Warnings, "No database directory found in backup")
	}

	// Validate backup structure
	a.validateBackupStructure(tempDir, analysis)

	return analysis, nil
}

// extractForAnalysis extracts only the metadata and database files (not the large filesystem files)
func (a *Analyzer) extractForAnalysis(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Only extract metadata and database directory files for analysis
		if header.Name == "metadata.json" || strings.HasPrefix(header.Name, "database/") {
			destPath := filepath.Join(destDir, header.Name)

			// Security check
			if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
				return fmt.Errorf("invalid file path: %s", header.Name)
			}

			switch header.Typeflag {
			case tar.TypeDir:
				if err := os.MkdirAll(destPath, os.FileMode(header.Mode)); err != nil {
					return err
				}

			case tar.TypeReg:
				if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
					return err
				}

				outFile, err := os.Create(destPath)
				if err != nil {
					return err
				}

				if _, err := io.Copy(outFile, tarReader); err != nil {
					outFile.Close()
					return err
				}
				outFile.Close()
			}
		} else {
			// Skip filesystem files for analysis (they can be very large)
			if _, err := io.Copy(io.Discard, tarReader); err != nil {
				return err
			}
		}
	}

	return nil
}

// analyzeDatabaseFiles analyzes the database JSON files to get counts
func (a *Analyzer) analyzeDatabaseFiles(dbDir string, analysis *BackupAnalysis) error {
	// Count users
	usersFile := filepath.Join(dbDir, "users.json")
	if users, err := a.countRecordsInFile(usersFile); err == nil {
		analysis.UserCount = users
	} else {
		analysis.Warnings = append(analysis.Warnings, "Could not count users: "+err.Error())
	}

	// Count API keys
	apiKeysFile := filepath.Join(dbDir, "api_keys.json")
	if apiKeys, err := a.countRecordsInFile(apiKeysFile); err == nil {
		analysis.APIKeyCount = apiKeys
	} else {
		analysis.Warnings = append(analysis.Warnings, "Could not count API keys: "+err.Error())
	}

	return nil
}

// countRecordsInFile counts the number of records in a JSON file
func (a *Analyzer) countRecordsInFile(filePath string) (int, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return 0, nil // File doesn't exist, count as 0
	}

	var records []interface{}
	if err := readJSON(filePath, &records); err != nil {
		return 0, err
	}

	return len(records), nil
}

// validateBackupStructure validates the structure of the backup
func (a *Analyzer) validateBackupStructure(tempDir string, analysis *BackupAnalysis) {
	// Check for required files/directories
	requiredPaths := []string{
		"metadata.json",
	}

	for _, path := range requiredPaths {
		fullPath := filepath.Join(tempDir, path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			analysis.Errors = append(analysis.Errors, fmt.Sprintf("Missing required file: %s", path))
			analysis.Valid = false
		}
	}

	// Check for database directory
	dbDir := filepath.Join(tempDir, "database")
	if stat, err := os.Stat(dbDir); err != nil || !stat.IsDir() {
		analysis.Warnings = append(analysis.Warnings, "No database directory found")
	}

	// If there are critical errors, mark as invalid
	if len(analysis.Errors) > 0 {
		analysis.Valid = false
	}
}
