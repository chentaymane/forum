package database

// seedCategories inserts the default categories once.
// INSERT OR IGNORE makes it a no-op if they already exist.
func seedCategories() error {
	categories := []string{"General", "Technology", "Art", "Science"}
	for _, name := range categories {
		if _, err := DB.Exec("INSERT OR IGNORE INTO categories (name) VALUES (?)", name); err != nil {
			return err
		}
	}
	return nil
}
