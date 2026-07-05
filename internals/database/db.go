package database

// ─── Database Initialisation ────────────────────────────────────────────────
// This package owns the single *sql.DB connection pool used by the entire
// application.  On startup it:
//   1. Opens the SQLite file (or creates it).
//   2. Reads and executes the schema.sql file to create tables.
//   3. Seeds a few default categories (General, Technology, Art, Science).
//   4. Runs a minimal migration to add profile columns if they don't exist.

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// DB is the shared database connection pool.
var DB *sql.DB

// InitDB opens the database, loads the schema, seeds categories, and runs
// any needed migrations.  Call this once at startup.
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
		DB.Close()
		return fmt.Errorf("failed to create tables: %w", err)
	}

	if err = DB.Ping(); err != nil {
		DB.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	if err = seedCategories(); err != nil {
		DB.Close()
		return fmt.Errorf("failed to seed categories: %w", err)
	}

	if err = migrateSchema(); err != nil {
		DB.Close()
		return fmt.Errorf("failed to migrate schema: %w", err)
	}

	fmt.Println("✓ Database initialized successfully")
	return nil
}

// loadSchema reads schema.sql and executes all the CREATE TABLE statements.
func loadSchema() error {
	schema := os.Getenv("SCHEMA_PATH")
	if schema == "" {
		schema = "./schema.sql"
	}
	content, err := os.ReadFile(schema)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	// SQLite needs foreign key enforcement enabled per‑connection
	if _, err := DB.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return err
	}
	_, err = DB.Exec(string(content))
	return err
}

// seedCategories inserts the default categories if they don't already exist.
func seedCategories() error {
	categories := []string{"General", "Technology", "Art", "Science"}
	for _, name := range categories {
		if _, err := DB.Exec("INSERT OR IGNORE INTO categories (name) VALUES (?)", name); err != nil {
			return err
		}
	}
	return nil
}

// migrateSchema adds columns that were added after the initial schema.
// When you run this project from scratch on a fresh database the columns
// will already exist, so these ALTER TABLE statements are effectively no‑ops.
func migrateSchema() error {
	columns := []struct {
		name string
		sql  string
	}{
		{name: "nickname", sql: "ALTER TABLE users ADD COLUMN nickname TEXT"},
		{name: "age", sql: "ALTER TABLE users ADD COLUMN age INTEGER"},
		{name: "gender", sql: "ALTER TABLE users ADD COLUMN gender TEXT"},
		{name: "first_name", sql: "ALTER TABLE users ADD COLUMN first_name TEXT"},
		{name: "last_name", sql: "ALTER TABLE users ADD COLUMN last_name TEXT"},
	}

	for _, col := range columns {
		// Only add the column if it doesn't already exist
		has, err := tableHasColumn("users", col.name)
		if err != nil {
			return err
		}
		if !has {
			if _, err := DB.Exec(col.sql); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
				return err
			}
		}
	}

	// Ensure private_messages table exists (may have been added later)
	_, err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS private_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			sender_id INTEGER NOT NULL,
			receiver_id INTEGER NOT NULL,
			content TEXT NOT NULL CHECK(length(content) > 0),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (sender_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (receiver_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	// Indexes for fast message lookups
	for _, idx := range []string{
		`CREATE INDEX IF NOT EXISTS idx_private_messages_pair_time ON private_messages(sender_id, receiver_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_private_messages_receiver_time ON private_messages(receiver_id, created_at)`,
	} {
		if _, err := DB.Exec(idx); err != nil {
			return err
		}
	}

	return nil
}

// tableHasColumn checks whether `tableName` has a column called `columnName`.
func tableHasColumn(tableName, columnName string) (bool, error) {
	rows, err := DB.Query("PRAGMA table_info(" + tableName + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if name == columnName {
			return true, nil
		}
	}
	return false, nil
}
