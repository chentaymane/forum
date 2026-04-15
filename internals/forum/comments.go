package forum

import (
	"database/sql"
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

	tx, err := database.DB.Begin()
	if err != nil {
		errors.RenderError(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	//  Check ownership
	var ownerID int
	err = tx.QueryRow("SELECT user_id FROM comments WHERE id = ?", commentID).Scan(&ownerID)
	if err != nil {
		errors.RenderError(w, "comment not found", http.StatusNotFound)
		return
	}

	if ownerID != userID {
		errors.RenderError(w, "Forbidden", http.StatusForbidden)
		return
	}

	//  Single delete (CASCADE handles everything else)
	result, err := tx.Exec("DELETE FROM comments WHERE id = ?", commentID)
	if err != nil {
		errors.RenderError(w, "Failed to delete comment", http.StatusInternalServerError)
		return
	}

	// optional safety check
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		errors.RenderError(w, "comment not found", http.StatusNotFound)
		return
	}

	if err = tx.Commit(); err != nil {
		errors.RenderError(w, "Failed to commit deletion", http.StatusInternalServerError)
		return
	}

	// Redirect back to the referring page (or home as fallback)
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

// CreateCommentHandler handles comment creation.
func CreateCommentHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserFromRequest(r)

	postID := r.FormValue("post_id")
	content := r.FormValue("content")

	if postID == "" || content == "" {
		errors.RenderError(w, "Post ID and content are required", http.StatusBadRequest)
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
	var rows *sql.Rows
	var err error
	var comments []Comment
	query := `
		SELECT 
			c.id, c.post_id, c.user_id, u.username, c.content, c.created_at,
			COALESCE(r.type, 0) AS reacted_to,
			COALESCE(rc.likes, 0) AS likes,
			COALESCE(rc.dislikes, 0) AS dislikes
		FROM comments c
		JOIN users u ON c.user_id = u.id
		LEFT JOIN reactions r 
			ON r.comment_id = c.id AND r.user_id = ?
		LEFT JOIN (
			SELECT 
				comment_id,
				SUM(CASE WHEN type = 1 THEN 1 ELSE 0 END) AS likes,
				SUM(CASE WHEN type = -1 THEN 1 ELSE 0 END) AS dislikes
			FROM reactions
			GROUP BY comment_id
		) rc ON c.id = rc.comment_id
		WHERE c.post_id = ?
		ORDER BY c.created_at ASC
	`
	rows, err = database.DB.Query(query, userID, postID)

	if err != nil {
		return comments, err
	}

	defer rows.Close()

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
