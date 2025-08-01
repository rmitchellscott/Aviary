package export

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rmitchellscott/aviary/internal/logging"
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
	ExtractionPath  string                 `json:"extraction_path,omitempty"` // Path if already extracted during analysis
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

// tryFastAnalysis attempts to read metadata.json if it's the first file in the archive
func (a *Analyzer) tryFastAnalysis(archivePath string) (*BackupAnalysis, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	var header *tar.Header
	for {
		h, err := tarReader.Next()
		if err != nil {
			return nil, err
		}
		
		if strings.HasPrefix(h.Name, "._") {
			continue
		}
		
		header = h
		break
	}

	if header.Name != "metadata.json" {
		logging.Logf("[ANALYZE] Archive uses old format (first file: %s), falling back to full extraction", header.Name)
		return nil, nil
	}

	logging.Logf("[ANALYZE] Found metadata.json as first file, parsing from fast path")
	
	// Parse metadata directly from the tar stream
	var metadata ExportMetadata
	if err := json.NewDecoder(tarReader).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode metadata: %w", err)
	}
	
	logging.Logf("[ANALYZE] Successfully parsed metadata.json from fast path")

	// Create analysis from metadata
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
		UserCount:       metadata.TotalUsers,    // Use from metadata if available
		APIKeyCount:     metadata.TotalAPIKeys,  // Use from metadata if available
	}

	// Check if this is an old backup without user/API key counts
	if metadata.TotalUsers == 0 && metadata.TotalAPIKeys == 0 && len(metadata.ExportedTables) > 0 {
		// Old backup format - add warning and return nil to trigger full extraction
		logging.Logf("[ANALYZE] Old backup format detected (missing user/API key counts), falling back to full extraction")
		return nil, nil
	}

	return analysis, nil
}

// AnalyzeBackup analyzes a backup file and returns metadata without restoring
func (a *Analyzer) AnalyzeBackup(archivePath string) (*BackupAnalysis, error) {
	logging.Logf("[ANALYZE] Attempting fast analysis for archive: %s", archivePath)
	
	// Try fast analysis first (for new archives with metadata.json first)
	if analysis, err := a.tryFastAnalysis(archivePath); err == nil && analysis != nil {
		logging.Logf("[ANALYZE] Fast analysis succeeded - using metadata.json from archive start")
		return analysis, nil
	} else if err != nil {
		logging.Logf("[ANALYZE] Fast analysis failed, falling back to full extraction: %v", err)
	} else {
		logging.Logf("[ANALYZE] Fast analysis returned nil (old archive format), falling back to full extraction")
	}
	// If fast analysis didn't work (old archive format or error), fall back to full extraction

	// Create permanent extraction directory for reuse (instead of temp directory)
	extractionsDir := filepath.Join(a.dataDir, "temp", "extractions")
	if err := os.MkdirAll(extractionsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create extractions directory: %w", err)
	}
	
	// Generate unique directory name for this analysis
	extractionID := fmt.Sprintf("analyze-%d", time.Now().UnixNano())
	extractionDir := filepath.Join(extractionsDir, extractionID)
	
	// Cleanup function for error cases
	cleanup := func() {
		os.RemoveAll(extractionDir)
	}

	// Extract entire archive for analysis
	if err := ExtractTarGz(archivePath, extractionDir); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to extract archive: %w", err)
	}

	// Read metadata
	metadataPath := filepath.Join(extractionDir, "metadata.json")
	var metadata ExportMetadata
	if err := readJSON(metadataPath, &metadata); err != nil {
		cleanup()
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
		ExtractionPath:  extractionDir, // Set extraction path for reuse
	}

	// Analyze database files
	dbDir := filepath.Join(extractionDir, "database")
	if stat, err := os.Stat(dbDir); err == nil && stat.IsDir() {
		if err := a.analyzeDatabaseFiles(dbDir, analysis); err != nil {
			analysis.Warnings = append(analysis.Warnings, "Failed to analyze database files: "+err.Error())
		}
	} else {
		analysis.Warnings = append(analysis.Warnings, "No database directory found in backup")
	}

	// Validate backup structure
	a.validateBackupStructure(extractionDir, analysis)

	// If analysis failed, cleanup the extraction directory
	if !analysis.Valid {
		cleanup()
		analysis.ExtractionPath = "" // Clear extraction path if invalid
	} else {
		logging.Logf("[ANALYZE] Full extraction completed, preserved at: %s", extractionDir)
	}

	return analysis, nil
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
