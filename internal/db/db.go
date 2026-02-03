package db

import (
	"database/sql"
	"embed"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/mattn/go-sqlite3"

	"github.com/emilianohg/anchorman/internal/config"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var db *sql.DB

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

	// Run migrations
	if err := runMigrations(db); err != nil {
		return nil, err
	}

	return db, nil
}

func Close() error {
	if db != nil {
		err := db.Close()
		db = nil
		return err
	}
	return nil
}

func runMigrations(db *sql.DB) error {
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return err
	}

	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", source, "sqlite3", driver)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}

func Get() *sql.DB {
	return db
}
