package forum

import (
	"net/http"

	"forum/internals/auth"
	"forum/internals/database"
	"forum/internals/errors"
)

// Comment represents a post comment.
type Comment struct {
	ID        int
	PostID    int
	UserID    int
	Username  string
	Content   string
	CreatedAt string
}

// CreateCommentHandler handles comment creation.
func CreateCommentHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserFromRequest(r)
	if err != nil {
		errors.RenderError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	postID := r.FormValue("post_id")
	content := r.FormValue("content")

	if postID == "" || content == "" {
		errors.RenderError(w, "Post ID and content are required", http.StatusBadRequest)
		return
	}

	_, err = database.DB.Exec(
		"INSERT INTO comments (post_id, user_id, content) VALUES (?, ?, ?)",
		postID, userID, content,
	)
	if err != nil {
		errors.RenderError(w, "Failed to create comment", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// GetCommentsByPost retrieves all comments for a specific post.
func GetCommentsByPost(postID int) ([]Comment, error) {
	rows, err := database.DB.Query(`
		SELECT c.id, c.post_id, c.user_id, u.username, c.content, c.created_at
		FROM comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.post_id = ?
		ORDER BY c.created_at ASC
	`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.PostID, &c.UserID, &c.Username, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}

	return comments, nil
}
