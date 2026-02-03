package db

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/mattn/go-sqlite3"

	"github.com/emilianohg/anchorman/internal/config"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var db *sql.DB

// MigrationStatus holds information about database migration state
type MigrationStatus struct {
	CurrentVersion uint
	LatestVersion  uint
	Dirty          bool
	Pending        bool
}

// Open opens the database connection without running migrations
func Open() (*sql.DB, error) {
	if db != nil {
		return db, nil
	}

	// Ensure directories exist
	if err := config.EnsureDirectories(); err != nil {
		return nil, err
	}

	dbPath, err := config.DatabasePath()
	if err != nil {
		return nil, err
	}

	db, err = sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, err
	}

	return db, nil
}

// OpenAndMigrate opens the database and runs all pending migrations
func OpenAndMigrate() (*sql.DB, error) {
	database, err := Open()
	if err != nil {
		return nil, err
	}

	if err := RunMigrations(); err != nil {
		return nil, err
	}

	return database, nil
}

func Close() error {
	if db != nil {
		err := db.Close()
		db = nil
		return err
	}
	return nil
}

// GetMigrationStatus returns the current migration status
func GetMigrationStatus() (*MigrationStatus, error) {
	if db == nil {
		return nil, fmt.Errorf("database not open")
	}

	m, err := getMigrator()
	if err != nil {
		return nil, err
	}

	// Get current version
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return nil, err
	}

	// Get latest available version by checking source
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, err
	}

	// Find the latest version
	var latestVersion uint
	first, err := source.First()
	if err == nil {
		latestVersion = first
		for {
			next, err := source.Next(latestVersion)
			if err != nil {
				break
			}
			latestVersion = next
		}
	}

	status := &MigrationStatus{
		CurrentVersion: version,
		LatestVersion:  latestVersion,
		Dirty:          dirty,
		Pending:        version < latestVersion,
	}

	return status, nil
}

// RunMigrations runs all pending migrations
func RunMigrations() error {
	if db == nil {
		return fmt.Errorf("database not open")
	}

	m, err := getMigrator()
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}

// getMigrator creates a new migrate instance
func getMigrator() (*migrate.Migrate, error) {
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return nil, err
	}

	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, err
	}

	return migrate.NewWithInstance("iofs", source, "sqlite3", driver)
}

func Get() *sql.DB {
	return db
}
