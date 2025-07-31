package database

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/logging"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Type     string // "sqlite" or "postgres"
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
	DataDir  string // For SQLite
}

// GetDatabaseConfig reads database configuration from environment variables
func GetDatabaseConfig() *DatabaseConfig {
	cfg := &DatabaseConfig{
		Type:     config.Get("DB_TYPE", "sqlite"),
		Host:     config.Get("DB_HOST", "localhost"),
		Port:     config.GetInt("DB_PORT", 5432),
		User:     config.Get("DB_USER", "aviary"),
		Password: config.Get("DB_PASSWORD", ""),
		DBName:   config.Get("DB_NAME", "aviary"),
		SSLMode:  config.Get("DB_SSLMODE", "disable"),
		DataDir:  config.Get("DATA_DIR", "/data"),
	}

	return cfg
}

// Initialize sets up the database connection and runs migrations
func Initialize() error {
	config := GetDatabaseConfig()

	var err error
	switch config.Type {
	case "postgres":
		DB, err = initPostgres(config)
	case "sqlite":
		DB, err = initSQLite(config)
	default:
		return fmt.Errorf("unsupported database type: %s", config.Type)
	}

	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Run auto-migration
	if err := runMigrations("STARTUP"); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize default system settings
	if err := initializeSystemSettings(); err != nil {
		return fmt.Errorf("failed to initialize system settings: %w", err)
	}

	logging.Logf("[STARTUP] Database initialized successfully (type: %s)", config.Type)
	return nil
}

// initPostgres initializes PostgreSQL connection
func initPostgres(config *DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		config.Host, config.User, config.Password, config.DBName, config.Port, config.SSLMode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: getGormLogger(),
	})
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

// initSQLite initializes SQLite connection
func initSQLite(config *DatabaseConfig) (*gorm.DB, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(config.DataDir, "aviary.db")

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: getGormLogger(),
	})
	if err != nil {
		return nil, err
	}

	// Configure SQLite settings
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(1) // SQLite doesn't support concurrent writes
	sqlDB.SetMaxIdleConns(1)

	// Enable foreign keys for SQLite
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		return nil, err
	}

	return db, nil
}

// runMigrations runs GORM auto-migration for all models
func runMigrations(logPrefix string) error {
	logging.Logf("[%s] Running GORM auto-migrations...", logPrefix)
	
	models := GetAllModels()

	// Force migration of all models
	for _, model := range models {
		if err := DB.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
	}

	// Add unique constraint for FolderCache
	if err := DB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_user_folder_path ON user_folders_cache (user_id, folder_path)").Error; err != nil {
		logging.Logf("[WARNING] failed to create unique index for folder cache: %v", err)
	}

	logging.Logf("[%s] GORM auto-migration completed successfully", logPrefix)
	return nil
}

// RunAutoMigrations runs GORM auto-migration for all models (public wrapper)
func RunAutoMigrations(logPrefix string) error {
	return runMigrations(logPrefix)
}

// initializeSystemSettings creates default system settings if they don't exist
func initializeSystemSettings() error {
	defaultSettings := map[string]SystemSetting{
		"smtp_enabled": {
			Key:         "smtp_enabled",
			Value:       "false",
			Description: "Whether SMTP is configured for password resets",
		},
		"smtp_host": {
			Key:         "smtp_host",
			Value:       "",
			Description: "SMTP server hostname",
		},
		"smtp_port": {
			Key:         "smtp_port",
			Value:       "587",
			Description: "SMTP server port",
		},
		"smtp_username": {
			Key:         "smtp_username",
			Value:       "",
			Description: "SMTP username",
		},
		"smtp_password": {
			Key:         "smtp_password",
			Value:       "",
			Description: "SMTP password",
		},
		"smtp_from": {
			Key:         "smtp_from",
			Value:       "",
			Description: "From email address for system emails",
		},
		"smtp_tls": {
			Key:         "smtp_tls",
			Value:       "true",
			Description: "Whether to use TLS for SMTP",
		},
		"registration_enabled": {
			Key:         "registration_enabled",
			Value:       "true",
			Description: "Whether new user registration is enabled",
		},
		"max_api_keys_per_user": {
			Key:         "max_api_keys_per_user",
			Value:       "10",
			Description: "Maximum API keys per user",
		},
		"password_reset_timeout_hours": {
			Key:         "password_reset_timeout_hours",
			Value:       "24",
			Description: "Password reset token timeout in hours",
		},
	}

	for _, setting := range defaultSettings {
		var existing SystemSetting
		if err := DB.First(&existing, "key = ?", setting.Key).Error; err == gorm.ErrRecordNotFound {
			if err := DB.Create(&setting).Error; err != nil {
				return fmt.Errorf("failed to create system setting %s: %w", setting.Key, err)
			}
		}
	}

	return nil
}

// getGormLogger returns appropriate GORM logger based on environment
func getGormLogger() logger.Interface {
	logLevel := logger.Warn
	if config.Get("GIN_MODE", "") == "debug" {
		logLevel = logger.Info
	}

	return logger.Default.LogMode(logLevel)
}

// Helper functions
// IsMultiUserMode checks if multi-user mode is enabled
func IsMultiUserMode() bool {
	return config.Get("MULTI_USER", "false") == "true"
}

// GetCurrentUser gets the current user from the database by ID
func GetCurrentUser(userID uuid.UUID) (*User, error) {
	var user User
	if err := DB.First(&user, "id = ? AND is_active = ?", userID, true).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername gets a user by username
func GetUserByUsername(username string) (*User, error) {
	var user User
	if err := DB.First(&user, "username = ? AND is_active = ?", username, true).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByOIDCSubject gets a user by OIDC subject
func GetUserByOIDCSubject(oidcSubject string) (*User, error) {
	var user User
	if err := DB.First(&user, "oidc_subject = ? AND is_active = ?", oidcSubject, true).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsernameWithoutOIDC gets a user by username without an OIDC subject
func GetUserByUsernameWithoutOIDC(username string) (*User, error) {
	var user User
	if err := DB.First(&user, "username = ? AND is_active = ? AND (oidc_subject IS NULL OR oidc_subject = '')", username, true).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByEmailWithoutOIDC gets a user by email without an OIDC subject
func GetUserByEmailWithoutOIDC(email string) (*User, error) {
	var user User
	if err := DB.First(&user, "email = ? AND is_active = ? AND (oidc_subject IS NULL OR oidc_subject = '')", email, true).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByEmail gets a user by email
func GetUserByEmail(email string) (*User, error) {
	var user User
	if err := DB.First(&user, "email = ? AND is_active = ?", email, true).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetSystemSetting gets a system setting by key
func GetSystemSetting(key string) (string, error) {
	var setting SystemSetting
	if err := DB.First(&setting, "key = ?", key).Error; err != nil {
		return "", err
	}
	return setting.Value, nil
}

// SetSystemSetting sets a system setting
func SetSystemSetting(key, value string, updatedBy *uuid.UUID) error {
	setting := SystemSetting{
		Key:       key,
		Value:     value,
		UpdatedBy: updatedBy,
		UpdatedAt: time.Now(),
	}

	return DB.Save(&setting).Error
}

// Close closes the database connection
func Close() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
