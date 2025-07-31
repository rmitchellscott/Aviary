package export

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/database"
	"github.com/rmitchellscott/aviary/internal/version"
	"gorm.io/gorm"
)

// ExportMetadata contains information about the export
type ExportMetadata struct {
	AviaryVersion   string    `json:"aviary_version"`
	GitCommit       string    `json:"git_commit"`
	ExportTimestamp time.Time `json:"export_timestamp"`
	DatabaseType    string    `json:"database_type"`
	UsersExported   []string  `json:"users_exported"`
	TotalDocuments  int64     `json:"total_documents"`
	TotalSizeBytes  int64     `json:"total_size_bytes"`
	ExportedTables  []string  `json:"exported_tables"`
}

// ExportOptions configures what to include in the export
type ExportOptions struct {
	IncludeDatabase bool
	IncludeFiles    bool
	IncludeConfigs  bool
	UserIDs         []uuid.UUID // If specified, only export these users
}

// ImportOptions configures how to handle the import
type ImportOptions struct {
	OverwriteFiles    bool
	OverwriteDatabase bool
	UserIDs           []uuid.UUID // If specified, only import these users
}

// Exporter handles creating complete backups
type Exporter struct {
	db              *gorm.DB
	dataDir         string
	totalDocuments  int64
}

// NewExporter creates a new exporter instance
func NewExporter(db *gorm.DB, dataDir string) *Exporter {
	return &Exporter{
		db:      db,
		dataDir: dataDir,
	}
}

// Export creates a complete backup archive
func (e *Exporter) Export(outputPath string, options ExportOptions) error {
	// Create temporary directory for staging
	tempDir, err := os.MkdirTemp("", "aviary-export-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Create directory structure
	dbDir := filepath.Join(tempDir, "database")
	fsDir := filepath.Join(tempDir, "filesystem")
	
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}
	if err := os.MkdirAll(fsDir, 0755); err != nil {
		return fmt.Errorf("failed to create filesystem directory: %w", err)
	}

	var metadata ExportMetadata
	var totalSize int64
	var exportedUsers []string

	// Export database if requested
	if options.IncludeDatabase {
		if err := e.exportDatabase(dbDir, &metadata, options); err != nil {
			return fmt.Errorf("failed to export database: %w", err)
		}
	}

	// Export filesystem data if requested
	if options.IncludeFiles || options.IncludeConfigs {
		size, users, err := e.exportFilesystem(fsDir, options)
		if err != nil {
			return fmt.Errorf("failed to export filesystem: %w", err)
		}
		totalSize += size
		exportedUsers = users
	}

	// Create metadata
	versionInfo := version.Get()
	config := database.GetDatabaseConfig()
	
	metadata.AviaryVersion = versionInfo.Version
	metadata.GitCommit = versionInfo.GitCommit
	metadata.ExportTimestamp = time.Now().UTC()
	metadata.DatabaseType = config.Type
	metadata.UsersExported = exportedUsers
	metadata.TotalSizeBytes = totalSize
	metadata.TotalDocuments = e.totalDocuments // Use actual file count

	// Write metadata
	metadataFile := filepath.Join(tempDir, "metadata.json")
	if err := writeJSON(metadataFile, metadata); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Create compressed archive
	if err := createTarGz(tempDir, outputPath); err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}

	return nil
}

// exportDatabase exports all database tables to JSON files
func (e *Exporter) exportDatabase(dbDir string, metadata *ExportMetadata, options ExportOptions) error {
	models := database.GetAllModels()
	var exportedTables []string

	for _, model := range models {
		tableName := getTableName(model)
		
		// Get all records for this model
		var records []map[string]interface{}
		
		query := database.DB
		
		// Filter by user ID if specified and model has UserID field
		if len(options.UserIDs) > 0 && hasUserIDField(model) {
			query = query.Where("user_id IN ?", options.UserIDs)
		}
		
		if err := query.Model(model).Find(&records).Error; err != nil {
			return fmt.Errorf("failed to export table %s: %w", tableName, err)
		}

		// Write to JSON file
		outputFile := filepath.Join(dbDir, tableName+".json")
		if err := writeJSON(outputFile, records); err != nil {
			return fmt.Errorf("failed to write %s: %w", tableName, err)
		}

		exportedTables = append(exportedTables, tableName)
	}

	metadata.ExportedTables = exportedTables
	
	return nil
}

// exportFilesystem exports user files and configurations
func (e *Exporter) exportFilesystem(fsDir string, options ExportOptions) (int64, []string, error) {
	var totalSize int64
	var exportedUsers []string
	var totalDocuments int64

	// Get list of users to export
	var users []database.User
	query := database.DB
	
	if len(options.UserIDs) > 0 {
		query = query.Where("id IN ?", options.UserIDs)
	}
	
	if err := query.Find(&users).Error; err != nil {
		return 0, nil, fmt.Errorf("failed to get users: %w", err)
	}

	for _, user := range users {
		userID := user.ID.String()
		exportedUsers = append(exportedUsers, userID)

		// Export user documents
		if options.IncludeFiles {
			userDocsDir := filepath.Join(e.dataDir, "users", userID, "pdfs")
			
			if stat, err := os.Stat(userDocsDir); err == nil && stat.IsDir() {
				destDir := filepath.Join(fsDir, "documents", userID)
				
				size, docCount, err := copyDirectoryAndCount(userDocsDir, destDir)
				if err != nil {
					return 0, nil, fmt.Errorf("failed to copy documents for user %s: %w", userID, err)
				}
				totalSize += size
				totalDocuments += docCount
			}
		}

		// Export user configs
		if options.IncludeConfigs {
			userConfigDir := filepath.Join(e.dataDir, "users", userID, "rmapi")
			
			if stat, err := os.Stat(userConfigDir); err == nil && stat.IsDir() {
				destDir := filepath.Join(fsDir, "configs", userID)
				
				size, _, err := copyDirectoryAndCount(userConfigDir, destDir)
				if err != nil {
					return 0, nil, fmt.Errorf("failed to copy configs for user %s: %w", userID, err)
				}
				totalSize += size
			}
		}
	}

	// Store the actual document count in metadata
	e.totalDocuments = totalDocuments

	return totalSize, exportedUsers, nil
}

// Helper functions

func getTableName(model interface{}) string {
	// This is a simplified version - you might want to use GORM's naming strategy
	switch model.(type) {
	case *database.User:
		return "users"
	case *database.APIKey:
		return "api_keys"
	case *database.UserSession:
		return "user_sessions"
	case *database.Document:
		return "documents"
	case *database.SystemSetting:
		return "system_settings"
	case *database.LoginAttempt:
		return "login_attempts"
	case *database.FolderCache:
		return "user_folders_cache"
	default:
		return "unknown"
	}
}

func hasUserIDField(model interface{}) bool {
	switch model.(type) {
	case *database.User, *database.SystemSetting, *database.LoginAttempt:
		return false
	default:
		return true
	}
}

func writeJSON(filename string, data interface{}) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func copyDirectory(src, dst string) (int64, error) {
	size, _, err := copyDirectoryAndCount(src, dst)
	return size, err
}

func copyDirectoryAndCount(src, dst string) (int64, int64, error) {
	var totalSize int64
	var fileCount int64

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		// Copy file
		if err := copyFile(path, destPath); err != nil {
			return err
		}
		
		totalSize += info.Size()
		// Count files (not directories)
		if !info.IsDir() {
			fileCount++
		}
		return nil
	})

	return totalSize, fileCount, err
}

func copyFile(src, dst string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Copy file permissions
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, sourceInfo.Mode())
}

// addFileToTar adds a single file to the tar archive
func addFileToTar(tarWriter *tar.Writer, filePath, nameInArchive string) error {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return err
	}

	// Create tar header
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = nameInArchive

	// Write header
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	// Write file contents
	_, err = io.Copy(tarWriter, file)
	return err
}

func createTarGz(sourceDir, outputPath string) error {
	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// First, add metadata.json if it exists
	metadataPath := filepath.Join(sourceDir, "metadata.json")
	if _, err := os.Stat(metadataPath); err == nil {
		if err := addFileToTar(tarWriter, metadataPath, "metadata.json"); err != nil {
			return fmt.Errorf("failed to add metadata.json: %w", err)
		}
	}

	// Walk through source directory
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path for tar
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Skip metadata.json as we've already added it
		if relPath == "metadata.json" {
			return nil
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// Write file contents for regular files
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(tarWriter, file)
			if err != nil {
				return err
			}
		}

		return nil
	})
}
