package forum

import (
	"forum/internals/auth"
	"forum/internals/database"
	"forum/internals/errors"
	"net/http"
	"strconv"
	"strings"
)

// DeletePostHandler handles the deletion of a post.
func DeletePost(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserFromRequest(r)

	postIDStr := r.FormValue("post_id")
	postID, err := strconv.Atoi(postIDStr)
	if err != nil {
		errors.RenderError(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		errors.RenderError(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	//  Check ownership
	var ownerID int
	err = tx.QueryRow("SELECT user_id FROM posts WHERE id = ?", postID).Scan(&ownerID)
	if err != nil {
		errors.RenderError(w, "Post not found", http.StatusNotFound)
		return
	}

	if ownerID != userID {
		errors.RenderError(w, "Forbidden", http.StatusForbidden)
		return
	}

	//  Single delete (CASCADE handles everything else)
	result, err := tx.Exec("DELETE FROM posts WHERE id = ?", postID)
	if err != nil {
		errors.RenderError(w, "Failed to delete post", http.StatusInternalServerError)
		return
	}

	// optional safety check
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		errors.RenderError(w, "Post not found", http.StatusNotFound)
		return
	}

	if err = tx.Commit(); err != nil {
		errors.RenderError(w, "Failed to commit deletion", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func CreatePost(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserFromRequest(r)
	title := strings.TrimSpace(r.FormValue("title"))
	content := strings.TrimSpace(r.FormValue("content"))
	categoryIDsStr := r.PostForm["categories"]

	if content == "" || title == "" || len(categoryIDsStr) == 0 {
		errors.RenderError(w, "Title and content and atleast 1 category are required", http.StatusBadRequest)
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		errors.RenderError(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Insert post
	res, err := tx.Exec("INSERT INTO posts (user_id, title, content) VALUES (?, ?, ?)", userID, title, content)
	if err != nil {
		errors.RenderError(w, "Failed to create post", http.StatusInternalServerError)
		return
	}

	postID, _ := res.LastInsertId()

	// Associate categories
	for _, catID := range categoryIDsStr {
		_, err = tx.Exec("INSERT INTO post_categories (post_id, category_id) VALUES (?, ?)", postID, catID)
		if err != nil {
			errors.RenderError(w, "Failed to associate categories", http.StatusInternalServerError)
			return
		}
	}

	if err = tx.Commit(); err != nil {
		errors.RenderError(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
