package database

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// InitDB opens the SQLite database and loads the schema.
func InitDB() error {
	var err error
	DB, err = sql.Open("sqlite", "./forum.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if _, err = DB.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return err
	}

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

func seedCategories() error {
	categories := []string{"General", "Technology", "Art", "Science"}
	for _, name := range categories {
		_, err := DB.Exec("INSERT OR IGNORE INTO categories (name) VALUES (?)", name)
		if err != nil {
			return err
		}
	}
	return nil
}
