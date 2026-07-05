package forum

import (
	"net/http"
	"strconv"

	"forum/internals/auth"
	"forum/internals/database"
	"forum/internals/errors"
)

// Comment represents a post comment.
type Comment struct {
	CommentID  int
	PostID     int
	UserID     int
	Username   string
	Content    string
	CreatedAt  string
	ComL       int
	ComD       int
	ReactedToC int
}

func DeleteCommentHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserFromRequest(r)

	commentIDStr := r.FormValue("comment_id")
	commentID, err := strconv.Atoi(commentIDStr)
	if err != nil {
		errors.RenderError(w, "Invalid comment ID", http.StatusBadRequest)
		return
	}

	deleteWithOwnershipCheck(w, r, "comments", "comment_id", commentID, userID, "comment")
}

// CreateCommentHandler handles comment creation.
func CreateCommentHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserFromRequest(r)

	postID := r.FormValue("post_id")
	content := r.FormValue("content")

	if postID == "" || content == "" || len(content) > 200 {
		errors.RenderError(w, "Invalid Input", http.StatusBadRequest)
		return
	}

	_, err := database.DB.Exec(
		"INSERT INTO comments (post_id, user_id, content) VALUES (?, ?, ?)",
		postID, userID, content,
	)
	if err != nil {
		errors.RenderError(w, "Failed to create comment", http.StatusInternalServerError)
		return
	}

	// Redirect back to the referring page (or home as fallback)
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

// GetCommentsByPost retrieves all comments for a specific post.
func GetCommentsByPost(userID, postID int) ([]Comment, error) {
	query := `
		SELECT c.id, c.post_id, c.user_id, u.username, c.content, c.created_at,
			COALESCE(r.type, 0) AS reacted_to,
			(SELECT COUNT(*) FROM reactions WHERE comment_id = c.id AND type = 1) AS likes,
			(SELECT COUNT(*) FROM reactions WHERE comment_id = c.id AND type = -1) AS dislikes
		FROM comments c
		JOIN users u ON c.user_id = u.id
		LEFT JOIN reactions r ON r.comment_id = c.id AND r.user_id = ?
		WHERE c.post_id = ?
		ORDER BY c.created_at ASC
	`
	rows, err := database.DB.Query(query, userID, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(
			&c.CommentID,
			&c.PostID,
			&c.UserID,
			&c.Username,
			&c.Content,
			&c.CreatedAt,
			&c.ReactedToC,
			&c.ComL,
			&c.ComD,
		); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}

	return comments, nil
}
