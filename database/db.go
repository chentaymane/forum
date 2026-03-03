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
	DB.Exec("PRAGMA foreign_keys = ON")

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	if err = createTables(); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	if err = seedCategories(); err != nil {
		return fmt.Errorf("failed to seed categories: %w", err)
	}

	if err = seedPosts(); err != nil {
		return fmt.Errorf("failed to seed posts: %w", err)
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

func createTables() error {
	usersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT UNIQUE NOT NULL,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL
	);`

	postsTable := `
	CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE	);`

	commentsTable := `
	CREATE TABLE IF NOT EXISTS comments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		post_id INTEGER NOT NULL,
		user_id INTEGER NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);`

	categoriesTable := `
	CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL
	);`

	postCategoriesTable := `
	CREATE TABLE IF NOT EXISTS post_categories (
		post_id INTEGER NOT NULL,
		category_id INTEGER NOT NULL,
		PRIMARY KEY (post_id, category_id),
		FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
		FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
	);`

	likesDislikesTable := `
	CREATE TABLE IF NOT EXISTS likes_dislikes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		post_id INTEGER,
		comment_id INTEGER,
		type INTEGER NOT NULL, -- 1 for like, -1 for dislike
		UNIQUE(user_id, post_id, comment_id),
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
		FOREIGN KEY (comment_id) REFERENCES comments(id) ON DELETE CASCADE
		CHECK ((post_id IS NOT NULL AND comment_id IS NULL) OR (post_id IS NULL AND comment_id IS NOT NULL))
	);`

	sessionsTable := `
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY, -- UUID
		user_id INTEGER NOT NULL,
		expires_at DATETIME NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE	);`

	queries := []string{
		usersTable,
		postsTable,
		commentsTable,
		categoriesTable,
		postCategoriesTable,
		likesDislikesTable,
		sessionsTable,
	}

	for _, query := range queries {
		_, err := DB.Exec(query)
		if err != nil {
			return fmt.Errorf("error executing query [%s]: %w", query, err)
		}
	}

	return nil
}

func seedPosts() error {
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM posts").Scan(&count)
	if err != nil {
		return err
	}
	if count >= 100 {
		return nil
	}

	fmt.Println("Seeding 100 dummy posts...")
	// Ensure dummy user
	_, err = DB.Exec("INSERT OR IGNORE INTO users (username, email, password) VALUES (?, ?, ?)", "admin", "admin@example.com", "$2a$14$V08M9Y.A.3jE0.D3b7W8r.C0F3F3F3F3F3F3F3F3F3F3F3F3F3")
	if err != nil {
		return err
	}

	var userID int
	err = DB.QueryRow("SELECT id FROM users WHERE username = 'admin'").Scan(&userID)
	if err != nil {
		return err
	}

	// Categories
	rows, err := DB.Query("SELECT id FROM categories")
	if err != nil {
		return err
	}
	defer rows.Close()
	var catIDs []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		catIDs = append(catIDs, id)
	}

	subjects := []string{"How to", "Why is", "Exploring", "The future of", "Deep dive into", "My thoughts on"}
	topics := []string{"Golang", "SQLite", "Web Dev", "Docker", "HTMX", "CSS Animation", "Quantum Computing", "Ancient History", "Cooking"}

	for i := count + 1; i <= 100; i++ {
		content := fmt.Sprintf("%s %s. This is dummy post number %d. It contains some interesting thoughts about development and more.", subjects[i%len(subjects)], topics[i%len(topics)], i)
		res, err := DB.Exec("INSERT INTO posts (user_id, content) VALUES (?, ?)", userID, content)
		if err != nil {
			continue
		}
		postID, _ := res.LastInsertId()
		if len(catIDs) > 0 {
			DB.Exec("INSERT INTO post_categories (post_id, category_id) VALUES (?, ?)", postID, catIDs[i%len(catIDs)])
		}
	}
	return nil
}
