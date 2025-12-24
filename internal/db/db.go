package db

import (
	"database/sql"
	_ "embed"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schema string

// DB wraps the database connection
type DB struct {
	*sql.DB
}

// New creates a new database connection and initializes the schema
func New() (*DB, error) {
	dbPath, err := getDBPath()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, err
	}

	// Initialize schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return &DB{db}, nil
}

// getDBPath returns the path to the database file
func getDBPath() (string, error) {
	// Use XDG data directory or fallback to home directory
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataDir = filepath.Join(home, ".local", "share")
	}

	appDir := filepath.Join(dataDir, "stm")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}

	return filepath.Join(appDir, "stm.db"), nil
}

// GetSetting retrieves a setting value by key
func (db *DB) GetSetting(key string) (string, error) {
	var value string
	err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSetting sets a setting value
func (db *DB) SetSetting(key, value string) error {
	_, err := db.Exec(`
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}
