package database

import (
    "context"      // <--- ENSURE THIS IS PRESENT
    "database/sql"
    "fmt"
    "io"           // <--- ENSURE THIS IS PRESENT
    "os"
    "path/filepath"

    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/sqlite3"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    _ "github.com/mattn/go-sqlite3"
    "github.com/rs/zerolog/log"
)

// DB wraps the sql.DB connection.
type DB struct {
	*sql.DB
}

// Connect initializes the database connection and runs migrations.
func Connect(dataSourceName string, migrationsPath string) (*DB, error) {
	// Ensure the directory for the database file exists
	dbDir := filepath.Dir(dataSourceName)
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory %s: %w", dbDir, err)
		}
		log.Info().Str("directory", dbDir).Msg("Created database directory")
	}


	db, err := sql.Open("sqlite3", dataSourceName+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pool (SQLite specific settings might be limited)
	db.SetMaxOpenConns(25) // Example value
	db.SetMaxIdleConns(5)  // Example value

	log.Info().Str("path", dataSourceName).Msg("Database connection established")

	// Run migrations
	if migrationsPath != "" {
		driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to create sqlite3 migrate driver: %w", err)
		}
		m, err := migrate.NewWithDatabaseInstance(
			fmt.Sprintf("file://%s", migrationsPath),
			"sqlite3", driver)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to init migrate instance: %w", err)
		}

		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			db.Close()
			return nil, fmt.Errorf("failed to apply migrations: %w", err)
		}
		log.Info().Msg("Database migrations applied successfully or no changes detected")
	} else {
		log.Warn().Msg("Migrations path not provided, skipping migrations.")
	}


	return &DB{db}, nil
}

// Backup creates a backup of the SQLite database.
func (db *DB) Backup(backupFilePath string) error {
	// SQLite .backup command is typically run via the sqlite3 CLI.
	// For in-app backup, you might copy the file, or use SQLite's online backup API.
	// For simplicity, this example just copies the file. Ensure DB is not actively written during this.
	// A better approach would be to use `sqlite3_backup_init`, `sqlite3_backup_step`, `sqlite3_backup_finish`
	// if you need an online backup without shelling out.
	
	// This is a naive file copy, not a proper online backup.
	// For a real app, use the SQLite Online Backup API or shell out to `sqlite3 .backup`.
	conn, err := db.DB.Conn(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get connection for backup: %w", err)
	}
	defer conn.Close()

	_, err = conn.ExecContext(context.Background(), fmt.Sprintf("VACUUM INTO '%s'", backupFilePath))
	if err != nil {
		return fmt.Errorf("failed to backup database to %s: %w", backupFilePath, err)
	}
	log.Info().Str("backup_path", backupFilePath).Msg("Database backup successful")
	return nil
}

// Restore restores the SQLite database from a backup file.
// WARNING: This will overwrite the current database.
func (db *DB) Restore(dataSourceName, backupFilePath string) error {
	// Close the current connection before restoring
	if err := db.Close(); err != nil {
		log.Warn().Err(err).Msg("Error closing current database connection before restore, proceeding cautiously.")
	}

	// Delete current database file
	if err := os.Remove(dataSourceName); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove current database file %s: %w", dataSourceName, err)
	}

	// Copy backup file to database file location
	sourceFile, err := os.Open(backupFilePath)
	if err != nil {
		return fmt.Errorf("failed to open backup file %s: %w", backupFilePath, err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dataSourceName)
	if err != nil {
		return fmt.Errorf("failed to create new database file %s: %w", dataSourceName, err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy backup to database file: %w", err)
	}
	
	log.Info().Str("backup_path", backupFilePath).Msg("Database restore successful. Please restart the application.")
	// The application would typically exit after a restore and require a restart
	// to reconnect to the newly restored database.
	return nil
}