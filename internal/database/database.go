package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"github.com/glebarez/sqlite"
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
	config := &DatabaseConfig{
		Type:    getEnv("DB_TYPE", "sqlite"),
		Host:    getEnv("DB_HOST", "localhost"),
		Port:    getEnvInt("DB_PORT", 5432),
		User:    getEnv("DB_USER", "aviary"),
		Password: getEnv("DB_PASSWORD", ""),
		DBName:  getEnv("DB_NAME", "aviary"),
		SSLMode: getEnv("DB_SSLMODE", "disable"),
		DataDir: getEnv("DATA_DIR", "/data"),
	}
	
	return config
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
	if err := runMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	
	// Initialize default system settings
	if err := initializeSystemSettings(); err != nil {
		return fmt.Errorf("failed to initialize system settings: %w", err)
	}
	
	log.Printf("Database initialized successfully (type: %s)", config.Type)
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
func runMigrations() error {
	models := GetAllModels()
	
	// Force migration of all models
	for _, model := range models {
		if err := DB.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
	}
	
	// Explicitly ensure OIDC column exists for existing databases
	if err := ensureOIDCColumn(); err != nil {
		log.Printf("Warning: failed to ensure OIDC column exists: %v", err)
	}
	
	// Add unique constraint for FolderCache
	if err := DB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_user_folder_path ON user_folders_cache (user_id, folder_path)").Error; err != nil {
		log.Printf("Warning: failed to create unique index for folder cache: %v", err)
	}
	
	return nil
}

// ensureOIDCColumn explicitly ensures the oidc_subject column exists
// This is needed because GORM AutoMigrate sometimes doesn't add new columns to existing tables
func ensureOIDCColumn() error {
	// Check if column exists by trying to select it
	var count int64
	err := DB.Model(&User{}).Where("oidc_subject IS NULL OR oidc_subject IS NOT NULL").Count(&count).Error
	
	if err != nil {
		// Column doesn't exist, try to add it
		log.Printf("OIDC column not found, attempting to add it...")
		
		// Use DB-specific syntax to add the column
		if DB.Dialector.Name() == "sqlite" {
			// SQLite syntax
			if err := DB.Exec("ALTER TABLE users ADD COLUMN oidc_subject VARCHAR(255)").Error; err != nil {
				// Column might already exist, check for specific error
				if !strings.Contains(err.Error(), "duplicate column name") && !strings.Contains(err.Error(), "already exists") {
					return fmt.Errorf("failed to add oidc_subject column: %w", err)
				}
			}
			
			// Add unique index
			if err := DB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_users_oidc_subject ON users(oidc_subject) WHERE oidc_subject IS NOT NULL").Error; err != nil {
				log.Printf("Warning: failed to create OIDC subject unique index: %v", err)
			}
		} else {
			// PostgreSQL syntax
			if err := DB.Exec("ALTER TABLE users ADD COLUMN IF NOT EXISTS oidc_subject VARCHAR(255) UNIQUE").Error; err != nil {
				return fmt.Errorf("failed to add oidc_subject column: %w", err)
			}
		}
		
		log.Printf("Successfully added oidc_subject column to users table")
	} else {
		log.Printf("OIDC column already exists in users table")
	}
	
	return nil
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
		"session_timeout_hours": {
			Key:         "session_timeout_hours",
			Value:       "24",
			Description: "Session timeout in hours",
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
	if os.Getenv("GIN_MODE") == "debug" {
		logLevel = logger.Info
	}
	
	return logger.Default.LogMode(logLevel)
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// IsMultiUserMode checks if multi-user mode is enabled
func IsMultiUserMode() bool {
	return getEnv("MULTI_USER", "false") == "true"
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