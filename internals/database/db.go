package database

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB() error {
	var err error
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./forum.db"
	}
	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err = loadSchema(); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	if err = seedCategories(); err != nil {
		return fmt.Errorf("failed to seed categories: %w", err)
	}

	fmt.Println("Database initialized successfully")
	return nil
}

func loadSchema() error {
	schema := os.Getenv("SCHEMA_PATH")
	if schema == "" {
		schema = "./schema.sql"
	}
	content, err := os.ReadFile(schema)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}
	if _, err := DB.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return err
	}

	_, err = DB.Exec(string(content))
	if err != nil {
		return fmt.Errorf("error executing query: %w", err)
	}
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
