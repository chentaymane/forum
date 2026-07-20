package forum

import (
	"net/http"

	"forum/internals/auth"
	"forum/internals/database"
)

// GetCategories returns all categories, used to fill the "new post" form.
func GetCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`SELECT id, name FROM categories ORDER BY id`)
	if err != nil {
		auth.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	categories := []Category{}
	for rows.Next() {
		var c Category
		if rows.Scan(&c.ID, &c.Name) == nil {
			categories = append(categories, c)
		}
	}
	auth.JSON(w, http.StatusOK, categories)
}
