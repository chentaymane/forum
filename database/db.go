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

	// Enable foreign keys in SQLite
	if _, err := DB.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return err
	}

	usersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT UNIQUE NOT NULL,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	postsTable := `
	CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		title TEXT NOT NULL CHECK(length(title) > 0),
		content TEXT NOT NULL CHECK(length(content) > 0),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);`

	commentsTable := `
	CREATE TABLE IF NOT EXISTS comments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		post_id INTEGER NOT NULL,
		user_id INTEGER NOT NULL,
		content TEXT NOT NULL CHECK(length(content) > 0),
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
		type INTEGER NOT NULL CHECK(type IN (1,-1)),

		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
		FOREIGN KEY (comment_id) REFERENCES comments(id) ON DELETE CASCADE,

		CHECK (
			(post_id IS NOT NULL AND comment_id IS NULL) OR
			(post_id IS NULL AND comment_id IS NOT NULL)
		)
	);`

	sessionsTable := `
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		expires_at DATETIME NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);`

	queries := []string{
		usersTable,
		postsTable,
		commentsTable,
		categoriesTable,
		postCategoriesTable,
		likesDislikesTable,
		sessionsTable,

		// performance indexes
		`CREATE INDEX IF NOT EXISTS idx_posts_user ON posts(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_comments_post ON comments(post_id);`,
		`CREATE INDEX IF NOT EXISTS idx_comments_user ON comments(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_post_categories_post ON post_categories(post_id);`,
		`CREATE INDEX IF NOT EXISTS idx_post_categories_category ON post_categories(category_id);`,
		`CREATE INDEX IF NOT EXISTS idx_likes_post ON likes_dislikes(post_id);`,
		`CREATE INDEX IF NOT EXISTS idx_likes_comment ON likes_dislikes(comment_id);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);`,

		// prevent duplicate reactions
		`CREATE UNIQUE INDEX IF NOT EXISTS unique_post_reaction
		ON likes_dislikes(user_id, post_id)
		WHERE comment_id IS NULL;`,

		`CREATE UNIQUE INDEX IF NOT EXISTS unique_comment_reaction
		ON likes_dislikes(user_id, comment_id)
		WHERE post_id IS NULL;`,

		`-- Posts trigger
		CREATE TRIGGER IF NOT EXISTS toggle_post_reaction
		BEFORE INSERT ON likes_dislikes
		FOR EACH ROW
		WHEN NEW.comment_id IS NULL
		BEGIN
			-- Case 1: same reaction exists → remove it and cancel insert
			DELETE FROM likes_dislikes
			WHERE user_id = NEW.user_id
			AND post_id = NEW.post_id
			AND comment_id IS NULL
			AND type = NEW.type;

			SELECT RAISE(IGNORE)
			WHERE changes() > 0;

			-- Case 2: opposite reaction exists → remove it (insert will continue)
			DELETE FROM likes_dislikes
			WHERE user_id = NEW.user_id
			AND post_id = NEW.post_id
			AND comment_id IS NULL;
		END;`,

		`CREATE TRIGGER IF NOT EXISTS toggle_comment_reaction
		BEFORE INSERT ON likes_dislikes
		FOR EACH ROW
		WHEN NEW.post_id IS NULL
		BEGIN
			DELETE FROM likes_dislikes
			WHERE user_id = NEW.user_id
			AND comment_id = NEW.comment_id
			AND post_id IS NULL
			AND type = NEW.type;

			SELECT RAISE(IGNORE)
			WHERE changes() > 0;

			DELETE FROM likes_dislikes
			WHERE user_id = NEW.user_id
			AND comment_id = NEW.comment_id
			AND post_id IS NULL;
		END;`,
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
		title := fmt.Sprintf("%s %s", subjects[i%len(subjects)], topics[i%len(topics)])
		content := fmt.Sprintf("%s. This is dummy post number %d. It contains some interesting thoughts about development and more.", title, i)
		res, err := DB.Exec("INSERT INTO posts (user_id, title, content) VALUES (?, ?, ?)", userID, title, content)
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
