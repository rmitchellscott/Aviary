package export

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/database"
	"github.com/rmitchellscott/aviary/internal/logging"
	"github.com/rmitchellscott/aviary/internal/storage"
	"gorm.io/gorm"
)

// Importer handles restoring from backup archives
type Importer struct {
	db             *gorm.DB
	dataDir        string
	storageBackend storage.StorageBackendWithInfo
}

// NewImporter creates a new importer instance
func NewImporter(db *gorm.DB, dataDir string) *Importer {
	return &Importer{
		db:             db,
		dataDir:        dataDir,
		storageBackend: storage.GetStorageBackend(),
	}
}

// Import restores from a backup archive
func (i *Importer) Import(archivePath string, options ImportOptions) (*ExportMetadata, error) {
	// Create temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "aviary-import-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract archive
	if err := ExtractTarGz(archivePath, tempDir); err != nil {
		return nil, fmt.Errorf("failed to extract archive: %w", err)
	}

	// Read metadata
	metadataPath := filepath.Join(tempDir, "metadata.json")
	var metadata ExportMetadata
	if err := readJSON(metadataPath, &metadata); err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	// Validate compatibility (optional - you might want version checks here)
	if err := i.validateMetadata(&metadata); err != nil {
		return nil, fmt.Errorf("backup validation failed: %w", err)
	}

	// Import database if present
	dbDir := filepath.Join(tempDir, "database")
	if _, err := os.Stat(dbDir); err == nil {
		if err := i.importDatabase(dbDir, options); err != nil {
			return nil, fmt.Errorf("failed to import database: %w", err)
		}
	}

	// Import filesystem if present
	fsDir := filepath.Join(tempDir, "filesystem")
	if _, err := os.Stat(fsDir); err == nil {
		if err := i.importFilesystem(fsDir, options); err != nil {
			return nil, fmt.Errorf("failed to import filesystem: %w", err)
		}
	}

	return &metadata, nil
}

// ImportFromExtractedDirectory imports from an already-extracted directory
func (i *Importer) ImportFromExtractedDirectory(extractedDir string, options ImportOptions) (*ExportMetadata, error) {
	// Read metadata
	metadataPath := filepath.Join(extractedDir, "metadata.json")
	var metadata ExportMetadata
	if err := readJSON(metadataPath, &metadata); err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	// Validate compatibility
	if err := i.validateMetadata(&metadata); err != nil {
		return nil, fmt.Errorf("backup validation failed: %w", err)
	}

	// Import database if present
	dbDir := filepath.Join(extractedDir, "database")
	if _, err := os.Stat(dbDir); err == nil {
		if err := i.importDatabase(dbDir, options); err != nil {
			return nil, fmt.Errorf("failed to import database: %w", err)
		}
	}

	// Import filesystem if present
	fsDir := filepath.Join(extractedDir, "filesystem")
	if _, err := os.Stat(fsDir); err == nil {
		if err := i.importFilesystem(fsDir, options); err != nil {
			return nil, fmt.Errorf("failed to import filesystem: %w", err)
		}
	}

	return &metadata, nil
}

// importDatabase imports all JSON files back to database tables
func (i *Importer) importDatabase(dbDir string, options ImportOptions) error {
	// Get the import order - dependencies first
	// Note: system_settings references users, so import users first, then update system_settings
	importOrder := []string{
		"users",
		"system_settings",
		"api_keys",
		"user_sessions",
		"documents",
		"user_folders_cache",
		"login_attempts",
	}

	for _, tableName := range importOrder {
		jsonFile := filepath.Join(dbDir, tableName+".json")

		// Skip if file doesn't exist
		if _, err := os.Stat(jsonFile); os.IsNotExist(err) {
			continue
		}

		if err := i.importTable(jsonFile, tableName, options); err != nil {
			return fmt.Errorf("failed to import table %s: %w", tableName, err)
		}
	}

	return nil
}

// cleanupExistingUserDirectories removes existing user files before restore
func (i *Importer) cleanupExistingUserDirectories(options ImportOptions) error {
	ctx := context.Background()
	
	// Get list of users to clean
	var userIDsToClean []string
	
	if len(options.UserIDs) > 0 {
		// Clean specific users
		for _, id := range options.UserIDs {
			userIDsToClean = append(userIDsToClean, id.String())
		}
	} else {
		// Get all users from database
		var users []database.User
		if err := database.DB.Find(&users).Error; err != nil {
			return fmt.Errorf("failed to get users for cleanup: %w", err)
		}
		for _, user := range users {
			userIDsToClean = append(userIDsToClean, user.ID.String())
		}
	}
	
	// Clean files for each user using the storage backend
	for _, userID := range userIDsToClean {
		userPrefix := fmt.Sprintf("users/%s/", userID)
		
		// List all files for this user
		keys, err := i.storageBackend.List(ctx, userPrefix)
		if err != nil {
			return fmt.Errorf("failed to list files for user %s: %w", userID, err)
		}
		
		// Delete all files
		for _, key := range keys {
			if err := i.storageBackend.Delete(ctx, key); err != nil {
				logging.Logf("[RESTORE] Warning: failed to delete %s: %v", key, err)
			}
		}
		
		if len(keys) > 0 {
			logging.Logf("[RESTORE] Cleaned up %d files for user %s", len(keys), userID)
		}
	}
	
	return nil
}

// importTable imports a specific table from JSON
func (i *Importer) importTable(jsonFile, tableName string, options ImportOptions) error {
	// Read JSON data
	var records []map[string]interface{}
	if err := readJSON(jsonFile, &records); err != nil {
		return fmt.Errorf("failed to read JSON file: %w", err)
	}

	if len(records) == 0 {
		return nil // Nothing to import
	}

	// Filter records by user ID if specified
	if len(options.UserIDs) > 0 && hasUserIDInTable(tableName) {
		records = filterRecordsByUserID(records, options.UserIDs)
	}

	// Get the appropriate model
	model := getModelForTable(tableName)
	if model == nil {
		return fmt.Errorf("unknown table: %s", tableName)
	}

	// Clear existing data (restore should replace everything)
	if len(options.UserIDs) > 0 && hasUserIDInTable(tableName) {
		// Only delete records for specific users
		if err := i.db.Where("user_id IN ?", options.UserIDs).Delete(model).Error; err != nil {
			return fmt.Errorf("failed to clear existing user data: %w", err)
		}
	} else {
		// Clear entire table for full restore using constraint-aware method
		if err := i.clearTableWithConstraintHandling(tableName, model); err != nil {
			return fmt.Errorf("failed to clear existing data: %w", err)
		}
	}

	// Import records in batches
	batchSize := 100
	for batchStart := 0; batchStart < len(records); batchStart += batchSize {
		end := batchStart + batchSize
		if end > len(records) {
			end = len(records)
		}

		batch := records[batchStart:end]
		if err := i.importBatch(batch, tableName); err != nil {
			return fmt.Errorf("failed to import batch: %w", err)
		}
	}

	return nil
}

// importBatch imports a batch of records
func (i *Importer) importBatch(records []map[string]interface{}, tableName string) error {
	switch tableName {
	case "users":
		return i.importUserBatch(records)
	case "api_keys":
		return i.importAPIKeyBatch(records)
	case "user_sessions":
		return i.importUserSessionBatch(records)
	case "documents":
		return i.importDocumentBatch(records)
	case "system_settings":
		return i.importSystemSettingBatch(records)
	case "login_attempts":
		return i.importLoginAttemptBatch(records)
	case "user_folders_cache":
		return i.importFolderCacheBatch(records)
	default:
		return fmt.Errorf("unsupported table: %s", tableName)
	}
}

// Model-specific batch import functions
func (i *Importer) importUserBatch(records []map[string]interface{}) error {
	var batch []database.User
	for _, record := range records {
		var user database.User

		// Handle User struct fields manually to preserve password hash integrity
		if err := mapUserRecord(record, &user); err != nil {
			return fmt.Errorf("failed to map user record: %w", err)
		}
		batch = append(batch, user)
	}
	return i.db.CreateInBatches(batch, len(batch)).Error
}

func (i *Importer) importAPIKeyBatch(records []map[string]interface{}) error {
	var batch []database.APIKey
	for _, record := range records {
		var apiKey database.APIKey
		if err := mapToStruct(record, &apiKey); err != nil {
			return err
		}
		batch = append(batch, apiKey)
	}
	return i.db.CreateInBatches(batch, len(batch)).Error
}

func (i *Importer) importUserSessionBatch(records []map[string]interface{}) error {
	var batch []database.UserSession
	for _, record := range records {
		var session database.UserSession
		if err := mapToStruct(record, &session); err != nil {
			return err
		}
		batch = append(batch, session)
	}
	return i.db.CreateInBatches(batch, len(batch)).Error
}

func (i *Importer) importDocumentBatch(records []map[string]interface{}) error {
	var batch []database.Document
	for _, record := range records {
		var doc database.Document
		if err := mapToStruct(record, &doc); err != nil {
			return err
		}
		batch = append(batch, doc)
	}
	return i.db.CreateInBatches(batch, len(batch)).Error
}

func (i *Importer) importSystemSettingBatch(records []map[string]interface{}) error {
	var batch []database.SystemSetting
	for _, record := range records {
		var setting database.SystemSetting
		if err := mapToStruct(record, &setting); err != nil {
			return err
		}
		// Clear the updated_by field since we don't care about preserving this info
		setting.UpdatedBy = nil
		batch = append(batch, setting)
	}
	return i.db.CreateInBatches(batch, len(batch)).Error
}

func (i *Importer) importLoginAttemptBatch(records []map[string]interface{}) error {
	var batch []database.LoginAttempt
	for _, record := range records {
		var attempt database.LoginAttempt
		if err := mapToStruct(record, &attempt); err != nil {
			return err
		}
		batch = append(batch, attempt)
	}
	return i.db.CreateInBatches(batch, len(batch)).Error
}

func (i *Importer) importFolderCacheBatch(records []map[string]interface{}) error {
	var batch []database.FolderCache
	for _, record := range records {
		var cache database.FolderCache
		if err := mapToStruct(record, &cache); err != nil {
			return err
		}
		batch = append(batch, cache)
	}
	return i.db.CreateInBatches(batch, len(batch)).Error
}

// importFilesystem restores user files and configurations
func (i *Importer) importFilesystem(fsDir string, options ImportOptions) error {
	// Clean up existing user directories first if overwriting
	if options.OverwriteFiles {
		if err := i.cleanupExistingUserDirectories(options); err != nil {
			return fmt.Errorf("failed to cleanup existing directories: %w", err)
		}
	}

	// Clean up existing backup files to prevent orphaned files after restore
	if err := i.cleanupExistingBackupDirectory(); err != nil {
		return fmt.Errorf("failed to cleanup existing backup directory: %w", err)
	}

	// Import documents
	docsDir := filepath.Join(fsDir, "documents")
	if _, err := os.Stat(docsDir); err == nil {
		if err := i.importUserFiles(docsDir, "pdfs", options); err != nil {
			return fmt.Errorf("failed to import documents: %w", err)
		}
	}

	// Import configs
	configsDir := filepath.Join(fsDir, "configs")
	if _, err := os.Stat(configsDir); err == nil {
		if err := i.populateDatabaseConfigsFromBackupFilesystem(configsDir, options); err != nil {
			logging.Logf("[RESTORE] Warning: failed to populate database configs from backup: %v", err)
		}
	}

	return nil
}

// cleanupExistingBackupDirectory removes all backup files from storage backend to prevent orphaned files after restore
func (i *Importer) cleanupExistingBackupDirectory() error {
	ctx := context.Background()
	
	// List all backup files in storage backend
	backupKeys, err := i.storageBackend.List(ctx, "backups/")
	if err != nil {
		return fmt.Errorf("failed to list backup files: %w", err)
	}
	
	// Delete each backup file
	for _, key := range backupKeys {
		if err := i.storageBackend.Delete(ctx, key); err != nil {
			logging.Logf("[RESTORE] Warning: failed to delete backup file %s: %v", key, err)
		}
	}
	
	if len(backupKeys) > 0 {
		logging.Logf("[RESTORE] Cleaned up %d backup files from storage backend", len(backupKeys))
	}

	return nil
}

// importUserFiles imports files for a specific subdirectory (pdfs or rmapi)
func (i *Importer) importUserFiles(sourceDir, subDir string, options ImportOptions) error {
	ctx := context.Background()
	
	// List user directories
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to read source directory %s: %w", sourceDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		userID := entry.Name()

		// Validate UUID format
		if _, err := uuid.Parse(userID); err != nil {
			continue
		}

		// Skip if not in the list of users to import
		if len(options.UserIDs) > 0 {
			found := false
			for _, id := range options.UserIDs {
				if id.String() == userID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		sourcePath := filepath.Join(sourceDir, userID)

		// Check source directory exists and has content
		sourceInfo, err := os.Stat(sourcePath)
		if err != nil {
			continue
		}
		if !sourceInfo.IsDir() {
			continue
		}

		// Import files from the extracted directory to storage backend
		err = i.importUserFilesToStorage(ctx, sourcePath, userID, subDir)
		if err != nil {
			return fmt.Errorf("failed to import files for user %s: %w", userID, err)
		}
	}

	return nil
}

// importUserFilesToStorage imports user files to the storage backend
func (i *Importer) importUserFilesToStorage(ctx context.Context, sourcePath, userID, subDir string) error {
	fileCount := 0
	
	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walk error at %s: %w", path, err)
		}
		
		// Skip directories
		if info.IsDir() {
			return nil
		}
		
		// Calculate relative path from source
		relPath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}
		
		// Create storage key
		storageKey := fmt.Sprintf("users/%s/%s/%s", userID, subDir, strings.ReplaceAll(relPath, "\\", "/"))
		
		// Open source file
		sourceFile, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open source file %s: %w", path, err)
		}
		defer sourceFile.Close()
		
		// Upload to storage backend
		err = i.storageBackend.Put(ctx, storageKey, sourceFile)
		if err != nil {
			return fmt.Errorf("failed to upload file %s to storage: %w", storageKey, err)
		}
		
		fileCount++
		if fileCount%10 == 0 {
			logging.Logf("[RESTORE] Imported %d files for user %s/%s", fileCount, userID, subDir)
		}
		
		return nil
	})
	
	if err != nil {
		return err
	}
	
	logging.Logf("[RESTORE] Successfully imported %d files for user %s/%s", fileCount, userID, subDir)
	return nil
}

// Helper functions

// clearUserForeignKeyReferences clears foreign key references to users table before deletion
func (i *Importer) clearUserForeignKeyReferences() error {
	// Clear updated_by references in system_settings since we don't care about preserving this info
	if err := i.db.Model(&database.SystemSetting{}).Where("updated_by IS NOT NULL").Update("updated_by", nil).Error; err != nil {
		return fmt.Errorf("failed to clear system_settings.updated_by references: %w", err)
	}
	logging.Logf("[RESTORE] Cleared system_settings.updated_by references to allow user table clearing")
	return nil
}

// clearTableWithConstraintHandling safely clears a table while handling foreign key constraints
func (i *Importer) clearTableWithConstraintHandling(tableName string, model interface{}) error {
	// For users table, always clear foreign key references first
	if tableName == "users" {
		logging.Logf("[RESTORE] Clearing foreign key references before deleting users")
		if err := i.clearUserForeignKeyReferences(); err != nil {
			return fmt.Errorf("failed to clear foreign key references: %w", err)
		}
	}

	// Clear the table
	return i.db.Where("1 = 1").Delete(model).Error
}

func (i *Importer) validateMetadata(metadata *ExportMetadata) error {
	// Add validation logic here
	currentConfig := database.GetDatabaseConfig()

	// Log warnings for version differences
	if metadata.AviaryVersion != "dev" {
		logging.Logf("[RESTORE] Importing backup from Aviary version %s", metadata.AviaryVersion)
	}

	if metadata.DatabaseType != currentConfig.Type {
		logging.Logf("[RESTORE] Backup database type (%s) differs from current (%s)",
			metadata.DatabaseType, currentConfig.Type)
	}

	return nil
}

func getModelForTable(tableName string) interface{} {
	switch tableName {
	case "users":
		return &database.User{}
	case "api_keys":
		return &database.APIKey{}
	case "user_sessions":
		return &database.UserSession{}
	case "documents":
		return &database.Document{}
	case "system_settings":
		return &database.SystemSetting{}
	case "login_attempts":
		return &database.LoginAttempt{}
	case "user_folders_cache":
		return &database.FolderCache{}
	default:
		return nil
	}
}

func hasUserIDInTable(tableName string) bool {
	switch tableName {
	case "users", "system_settings", "login_attempts":
		return false
	default:
		return true
	}
}

func filterRecordsByUserID(records []map[string]interface{}, userIDs []uuid.UUID) []map[string]interface{} {
	var filtered []map[string]interface{}

	userIDStrs := make(map[string]bool)
	for _, id := range userIDs {
		userIDStrs[id.String()] = true
	}

	for _, record := range records {
		if userIDVal, exists := record["user_id"]; exists {
			if userIDStr, ok := userIDVal.(string); ok {
				if userIDStrs[userIDStr] {
					filtered = append(filtered, record)
				}
			}
		}
	}

	return filtered
}

func readJSON(filename string, v interface{}) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	return decoder.Decode(v)
}

func mapToStruct(data map[string]interface{}, result interface{}) error {
	// Use JSON marshaling but without strict field validation to handle schema changes
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Use standard unmarshal without strict validation - this allows unknown fields
	if err := json.Unmarshal(jsonBytes, result); err != nil {
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return nil
}

// mapUserRecord maps user data without double JSON encoding to preserve password hashes
func mapUserRecord(data map[string]interface{}, user *database.User) error {
	// Parse UUID fields
	if idStr, ok := data["id"].(string); ok {
		if id, err := uuid.Parse(idStr); err == nil {
			user.ID = id
		}
	}

	// Handle string fields directly to avoid encoding issues
	if username, ok := data["username"].(string); ok {
		user.Username = username
	}
	if email, ok := data["email"].(string); ok {
		user.Email = email
	}

	// CRITICAL: Handle password hash directly to avoid corruption
	if password, ok := data["password"].(string); ok {
		user.Password = password // Direct assignment preserves exact hash
	}

	// Handle boolean fields
	if isAdmin, ok := data["is_admin"].(bool); ok {
		user.IsAdmin = isAdmin
	}
	if isActive, ok := data["is_active"].(bool); ok {
		user.IsActive = isActive
	}
	// Skip email_verified field (removed from model)

	// Handle optional string fields
	if rmapiHost, ok := data["rmapi_host"].(string); ok {
		user.RmapiHost = rmapiHost
	}
	if defaultRmdir, ok := data["default_rmdir"].(string); ok {
		user.DefaultRmdir = defaultRmdir
	}
	if coverpageSetting, ok := data["coverpage_setting"].(string); ok {
		user.CoverpageSetting = coverpageSetting
	}
	if resetToken, ok := data["reset_token"].(string); ok {
		user.ResetToken = resetToken
	}
	if rmapiConfig, ok := data["rmapi_config"].(string); ok {
		user.RmapiConfig = rmapiConfig
	}
	// Skip verification_token field (removed from model)

	// Handle integer fields
	if folderRefreshPercent, ok := data["folder_refresh_percent"].(float64); ok {
		user.FolderRefreshPercent = int(folderRefreshPercent)
	}

	// Handle time fields
	if createdAtStr, ok := data["created_at"].(string); ok {
		if createdAt, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
			user.CreatedAt = createdAt
		}
	}
	if updatedAtStr, ok := data["updated_at"].(string); ok {
		if updatedAt, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
			user.UpdatedAt = updatedAt
		}
	}
	if lastLoginStr, ok := data["last_login"].(string); ok && lastLoginStr != "" {
		if lastLogin, err := time.Parse(time.RFC3339, lastLoginStr); err == nil {
			user.LastLogin = &lastLogin
		}
	}
	if resetExpiresStr, ok := data["reset_token_expires"].(string); ok && resetExpiresStr != "" {
		if resetExpires, err := time.Parse(time.RFC3339, resetExpiresStr); err == nil {
			user.ResetTokenExpires = resetExpires
		}
	}

	logging.Logf("[RESTORE] Mapped user %s", user.Username)
	return nil
}

func ExtractTarGz(archivePath, destDir string) error {
	// Open archive file
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Calculate destination path
		destPath := filepath.Join(destDir, header.Name)

		// Ensure the destination is within destDir (security check)
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(destPath, os.FileMode(header.Mode)); err != nil {
				return err
			}

		case tar.TypeReg:
			// Create file
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

			// Set file permissions
			if err := os.Chmod(destPath, os.FileMode(header.Mode)); err != nil {
				return err
			}
		}
	}

	return nil
}

// Enhanced copy function with logging and validation
func copyDirectoryContentsWithLogging(src, dst string, overwrite bool) (int, error) {
	fileCount := 0

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walk error at %s: %w", path, err)
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}

		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			if err := os.MkdirAll(destPath, info.Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", destPath, err)
			}
			return nil
		}

		// Check if file exists
		if _, err := os.Stat(destPath); err == nil && !overwrite {
			return nil
		}

		if err := copyFileWithValidation(path, destPath); err != nil {
			return fmt.Errorf("failed to copy file %s to %s: %w", path, destPath, err)
		}

		fileCount++
		return nil
	})

	return fileCount, err
}

func copyFileWithValidation(src, dst string) error {
	// Check source file exists and is readable
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("source file not accessible: %w", err)
	}

	if srcInfo.IsDir() {
		return fmt.Errorf("source is a directory, expected file")
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Copy file
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	written, err := io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Verify file size matches
	if written != srcInfo.Size() {
		return fmt.Errorf("file size mismatch: wrote %d bytes, expected %d", written, srcInfo.Size())
	}

	// Copy permissions
	if err := os.Chmod(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}

// populateDatabaseConfigsFromBackupFilesystem reads rmapi configs from backup and populates database
func (i *Importer) populateDatabaseConfigsFromBackupFilesystem(configsDir string, options ImportOptions) error {
	// Only populate in multi-user mode
	if !database.IsMultiUserMode() {
		return nil
	}

	// Get list of users to process
	var users []database.User
	query := database.DB
	
	if len(options.UserIDs) > 0 {
		query = query.Where("id IN ?", options.UserIDs)
	}
	
	if err := query.Find(&users).Error; err != nil {
		return fmt.Errorf("failed to get users: %w", err)
	}

	populatedCount := 0
	for _, user := range users {
		userID := user.ID.String()
		
		// Check for rmapi config in backup filesystem section
		userConfigDir := filepath.Join(configsDir, userID)
		configPath := filepath.Join(userConfigDir, "rmapi.conf")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			continue
		}

		// Read config content directly from backup
		configContent, err := os.ReadFile(configPath)
		if err != nil {
			logging.Logf("[RESTORE] Warning: failed to read config from backup for user %s: %v", user.Username, err)
			continue
		}

		// Update user with config content in database
		if err := database.DB.Model(&user).Update("rmapi_config", string(configContent)).Error; err != nil {
			logging.Logf("[RESTORE] Warning: failed to save config to database for user %s: %v", user.Username, err)
			continue
		}

		populatedCount++
		logging.Logf("[RESTORE] Populated database config from backup for user %s", user.Username)
	}

	if populatedCount > 0 {
		logging.Logf("[RESTORE] Populated %d rmapi configs from backup directly to database", populatedCount)
	}

	return nil
}

// populateDatabaseConfigsFromFilesystem reads restored rmapi configs from filesystem
// and populates the database rmapi_config field for users
func (i *Importer) populateDatabaseConfigsFromFilesystem(options ImportOptions) error {
	// Only populate in multi-user mode
	if !database.IsMultiUserMode() {
		return nil
	}

	// Get list of users to process
	var users []database.User
	query := database.DB
	
	if len(options.UserIDs) > 0 {
		query = query.Where("id IN ?", options.UserIDs)
	}
	
	if err := query.Find(&users).Error; err != nil {
		return fmt.Errorf("failed to get users: %w", err)
	}

	populatedCount := 0
	for _, user := range users {
		userID := user.ID.String()
		
		// Check for restored filesystem config
		configPath := filepath.Join(i.dataDir, "users", userID, "rmapi", "rmapi.conf")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			continue
		}

		// Read config content
		configContent, err := os.ReadFile(configPath)
		if err != nil {
			logging.Logf("[RESTORE] Warning: failed to read config for user %s: %v", user.Username, err)
			continue
		}

		// Update user with config content in database
		if err := database.DB.Model(&user).Update("rmapi_config", string(configContent)).Error; err != nil {
			logging.Logf("[RESTORE] Warning: failed to save config to database for user %s: %v", user.Username, err)
			continue
		}

		// Clean up filesystem rmapi directory after successful database save
		rmapiDir := filepath.Join(i.dataDir, "users", userID, "rmapi")
		if err := os.RemoveAll(rmapiDir); err != nil {
			logging.Logf("[RESTORE] Warning: failed to cleanup rmapi directory for user %s: %v", user.Username, err)
		} else {
			logging.Logf("[RESTORE] Cleaned up filesystem rmapi config for user %s", user.Username)
		}

		populatedCount++
		logging.Logf("[RESTORE] Populated database config for user %s", user.Username)
	}

	if populatedCount > 0 {
		logging.Logf("[RESTORE] Populated %d rmapi configs from filesystem to database", populatedCount)
	}

	return nil
}

// Legacy function for backward compatibility - just calls the new implementation
func copyDirectoryContents(src, dst string, overwrite bool) error {
	_, err := copyDirectoryContentsWithLogging(src, dst, overwrite)
	return err
}
