package export

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/database"
	"gorm.io/gorm"
)

// Importer handles restoring from backup archives
type Importer struct {
	db      *gorm.DB
	dataDir string
}

// NewImporter creates a new importer instance
func NewImporter(db *gorm.DB, dataDir string) *Importer {
	return &Importer{
		db:      db,
		dataDir: dataDir,
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
	if err := extractTarGz(archivePath, tempDir); err != nil {
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

// importDatabase imports all JSON files back to database tables
func (i *Importer) importDatabase(dbDir string, options ImportOptions) error {
	// Get the import order - dependencies first
	importOrder := []string{
		"system_settings",
		"users",
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

// cleanupExistingUserDirectories removes existing user directories before restore
func (i *Importer) cleanupExistingUserDirectories(options ImportOptions) error {
	// Get list of existing users in the data directory
	usersDir := filepath.Join(i.dataDir, "users")
	if _, err := os.Stat(usersDir); os.IsNotExist(err) {
		// No users directory exists, nothing to clean up
		return nil
	}

	entries, err := os.ReadDir(usersDir)
	if err != nil {
		return fmt.Errorf("failed to read users directory: %w", err)
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

		// If specific users are specified, only clean those
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

		// Remove the entire user directory to ensure clean restore
		userDir := filepath.Join(usersDir, userID)
		if err := os.RemoveAll(userDir); err != nil {
			return fmt.Errorf("failed to remove existing directory for user %s: %w", userID, err)
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
		// Clear entire table for full restore
		if err := i.db.Where("1 = 1").Delete(model).Error; err != nil {
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
		if err := i.importUserFiles(configsDir, "rmapi", options); err != nil {
			return fmt.Errorf("failed to import configs: %w", err)
		}
	}

	return nil
}

// importUserFiles imports files for a specific subdirectory (pdfs or rmapi)
func (i *Importer) importUserFiles(sourceDir, subDir string, options ImportOptions) error {
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
		destPath := filepath.Join(i.dataDir, "users", userID, subDir)

		// Check source directory exists and has content
		sourceInfo, err := os.Stat(sourcePath)
		if err != nil {
			continue
		}
		if !sourceInfo.IsDir() {
			continue
		}

		// Create destination directory with proper permissions
		if err := os.MkdirAll(destPath, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory %s: %w", destPath, err)
		}

		// Copy files with detailed logging
		_, err = copyDirectoryContentsWithLogging(sourcePath, destPath, true) // Always overwrite during restore
		if err != nil {
			return fmt.Errorf("failed to copy files for user %s: %w", userID, err)
		}
	}

	return nil
}

// Helper functions

func (i *Importer) validateMetadata(metadata *ExportMetadata) error {
	// Add validation logic here
	currentConfig := database.GetDatabaseConfig()
	
	// Log warnings for version differences
	if metadata.AviaryVersion != "dev" {
		fmt.Printf("Import: Importing backup from Aviary version %s\n", metadata.AviaryVersion)
	}
	
	if metadata.DatabaseType != currentConfig.Type {
		fmt.Printf("Import: Warning - Backup database type (%s) differs from current (%s)\n", 
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
	if emailVerified, ok := data["email_verified"].(bool); ok {
		user.EmailVerified = emailVerified
	}
	
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
	if verificationToken, ok := data["verification_token"].(string); ok {
		user.VerificationToken = verificationToken
	}
	
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
	
	fmt.Printf("Import: Mapped user %s\n", user.Username)
	return nil
}

func extractTarGz(archivePath, destDir string) error {
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

// Legacy function for backward compatibility - just calls the new implementation
func copyDirectoryContents(src, dst string, overwrite bool) error {
	_, err := copyDirectoryContentsWithLogging(src, dst, overwrite)
	return err
}
