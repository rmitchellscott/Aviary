package database

import (
	"embed"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations
var migrationFS embed.FS

// RunMigrations runs any pending database migrations using golang-migrate
func RunMigrations() error {
	config := GetDatabaseConfig()

	log.Println("Running database migrations...")

	// Create source from embedded filesystem
	d, err := iofs.New(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	// Get underlying *sql.DB
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying database: %w", err)
	}

	// Create appropriate database driver
	var m *migrate.Migrate
	switch config.Type {
	case "postgres":
		pg, err := postgres.WithInstance(sqlDB, &postgres.Config{})
		if err != nil {
			return fmt.Errorf("failed to create postgres driver: %w", err)
		}
		m, err = migrate.NewWithInstance("iofs", d, "postgres", pg)

	case "sqlite":
		// Use sqlite3.WithInstance with the existing pure-Go sqlite connection
		sqlite, err := sqlite3.WithInstance(sqlDB, &sqlite3.Config{})
		if err != nil {
			return fmt.Errorf("failed to create sqlite driver: %w", err)
		}
		m, err = migrate.NewWithInstance("iofs", d, "sqlite3", sqlite)

	default:
		return fmt.Errorf("unsupported database type: %s", config.Type)
	}
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	// Run migrations
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if err == migrate.ErrNoChange {
		log.Println("No migrations to run")
	} else {
		log.Println("Migrations completed successfully")
	}

	return nil
}
