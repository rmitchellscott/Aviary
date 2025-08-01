package database

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rmitchellscott/aviary/internal/logging"
	"gorm.io/gorm"
)

// RunMigrations runs any pending database migrations using gormigrate
func RunMigrations(logPrefix string) error {
	logging.Logf("[%s] Running database migrations...", logPrefix)

	// Create migrator with our migrations
	m := gormigrate.New(DB, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "202507270001_add_cascade_foreign_keys",
			Migrate: func(tx *gorm.DB) error {
				// Add CASCADE to foreign key constraints to fix restore issues
				// GORM handles database differences automatically
				
				// Drop existing constraint and recreate with CASCADE for backup_jobs
				if tx.Migrator().HasConstraint(&BackupJob{}, "admin_user_id") {
					if err := tx.Migrator().DropConstraint(&BackupJob{}, "admin_user_id"); err != nil {
						logging.Logf("[INFO] Could not drop existing backup_jobs constraint: %v", err)
					}
				}
				if err := tx.Migrator().CreateConstraint(&BackupJob{}, "AdminUser"); err != nil {
					logging.Logf("[WARNING] Could not create CASCADE constraint for backup_jobs: %v", err)
				}
				
				// Drop existing constraint and recreate with CASCADE for restore_uploads  
				if tx.Migrator().HasConstraint(&RestoreUpload{}, "admin_user_id") {
					if err := tx.Migrator().DropConstraint(&RestoreUpload{}, "admin_user_id"); err != nil {
						logging.Logf("[INFO] Could not drop existing restore_uploads constraint: %v", err)
					}
				}
				if err := tx.Migrator().CreateConstraint(&RestoreUpload{}, "AdminUser"); err != nil {
					logging.Logf("[WARNING] Could not create CASCADE constraint for restore_uploads: %v", err)
				}
				
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				// Remove CASCADE constraints (rollback to non-CASCADE)
				if tx.Migrator().HasConstraint(&BackupJob{}, "AdminUser") {
					tx.Migrator().DropConstraint(&BackupJob{}, "AdminUser")
				}
				if tx.Migrator().HasConstraint(&RestoreUpload{}, "AdminUser") {
					tx.Migrator().DropConstraint(&RestoreUpload{}, "AdminUser")
				}
				return nil
			},
		},
		{
			ID: "202508010001_add_restore_extraction_jobs",
			Migrate: func(tx *gorm.DB) error {
				// Create the restore_extraction_jobs table
				if err := tx.AutoMigrate(&RestoreExtractionJob{}); err != nil {
					return fmt.Errorf("failed to create restore_extraction_jobs table: %w", err)
				}
				
				// Ensure foreign key constraints are created with CASCADE
				if err := tx.Migrator().CreateConstraint(&RestoreExtractionJob{}, "AdminUser"); err != nil {
					logging.Logf("[WARNING] Could not create CASCADE constraint for restore_extraction_jobs.admin_user_id: %v", err)
				}
				
				if err := tx.Migrator().CreateConstraint(&RestoreExtractionJob{}, "RestoreUpload"); err != nil {
					logging.Logf("[WARNING] Could not create CASCADE constraint for restore_extraction_jobs.restore_upload_id: %v", err)
				}
				
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				// Drop the restore_extraction_jobs table
				return tx.Migrator().DropTable(&RestoreExtractionJob{})
			},
		},
	})

	// Set initial schema if this is a fresh database
	m.InitSchema(func(tx *gorm.DB) error {
		// AutoMigrate all models to set up initial schema
		models := GetAllModels()
		for _, model := range models {
			if err := tx.AutoMigrate(model); err != nil {
				return fmt.Errorf("failed to migrate %T: %w", model, err)
			}
		}
		return nil
	})

	// Run migrations
	if err := m.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	logging.Logf("[%s] Migrations completed successfully", logPrefix)
	return nil
}
