package database

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

// DB is the shared database handle used across the whole app.
var DB *sql.DB

// InitDB opens the SQLite database, loads the schema and seeds the categories.
func InitDB() error {
	var err error
	DB, err = sql.Open("sqlite", "./forum.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Enforce foreign keys so deletes cascade correctly.
	if _, err = DB.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return err
	}

	// Create the tables from schema.sql (uses IF NOT EXISTS, so it's safe to rerun).
	content, err := os.ReadFile("./schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}
	if _, err = DB.Exec(string(content)); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	if err = seedCategories(); err != nil {
		return fmt.Errorf("failed to seed categories: %w", err)
	}

	fmt.Println("Database initialized successfully")
	return nil
}
