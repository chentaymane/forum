package forum

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"forum/internals/auth"
	"forum/internals/database"
)

// CommentsHandler lists a post's comments (GET), creates one (POST), or deletes one (POST with action=delete).
func CommentsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getComments(w, r)
	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			auth.Error(w, http.StatusBadRequest, "invalid request")
			return
		}
		var probe struct {
			Action string `json:"action"`
		}
		if json.Unmarshal(body, &probe) == nil && probe.Action == "delete" {
			deleteComment(w, r, body)
			return
		}
		createCommentFromBody(w, r, body)
	default:
		auth.Error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// getComments returns the comments of a post (?post_id=<id>), oldest first,
// each with its like/dislike counts and the current user's reaction.
func getComments(w http.ResponseWriter, r *http.Request) {
	postID, _ := strconv.Atoi(r.URL.Query().Get("post_id"))
	rows, err := database.DB.Query(`
		SELECT c.id, c.user_id, u.nickname, c.content, substr(c.created_at, 1, 16),
			(SELECT COUNT(*) FROM reactions WHERE comment_id = c.id AND type = 1),
			(SELECT COUNT(*) FROM reactions WHERE comment_id = c.id AND type = -1),
			COALESCE((SELECT type FROM reactions WHERE comment_id = c.id AND user_id = ?), 0)
		FROM comments c
		JOIN users u ON u.id = c.user_id
		WHERE c.post_id = ?
		ORDER BY c.id`, auth.UserID(r), postID)
	if err != nil {
		auth.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	comments := []Comment{}
	for rows.Next() {
		var c Comment
		if rows.Scan(&c.ID, &c.UserID, &c.Nickname, &c.Content, &c.Date, &c.Likes, &c.Dislikes, &c.ReactedTo) == nil {
			comments = append(comments, c)
		}
	}
	auth.JSON(w, http.StatusOK, comments)
}

// createCommentFromBody inserts a comment on a post.
func createCommentFromBody(w http.ResponseWriter, r *http.Request, body []byte) {
	var in struct {
		PostID  int    `json:"postId"`
		Content string `json:"content"`
	}
	if json.Unmarshal(body, &in) != nil {
		auth.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	in.Content = strings.TrimSpace(in.Content)
	if in.PostID < 1 || in.Content == "" {
		auth.Error(w, http.StatusBadRequest, "comment is empty")
		return
	}
	if len(in.Content) > auth.MaxCommentLen {
		auth.Error(w, http.StatusBadRequest, "comment too long")
		return
	}

	_, err := database.DB.Exec(`INSERT INTO comments (post_id, user_id, content) VALUES (?, ?, ?)`,
		in.PostID, auth.UserID(r), in.Content)
	if err != nil {
		auth.Error(w, http.StatusInternalServerError, "failed to create comment")
		return
	}
	auth.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// deleteComment removes a comment if the requester is its owner.
func deleteComment(w http.ResponseWriter, r *http.Request, body []byte) {
	var in struct {
		ID int `json:"id"`
	}
	if json.Unmarshal(body, &in) != nil || in.ID < 1 {
		auth.Error(w, http.StatusBadRequest, "invalid comment id")
		return
	}
	userID := auth.UserID(r)
	var ownerID int
	if err := database.DB.QueryRow(`SELECT user_id FROM comments WHERE id = ?`, in.ID).Scan(&ownerID); err != nil {
		auth.Error(w, http.StatusNotFound, "comment not found")
		return
	}
	if ownerID != userID {
		auth.Error(w, http.StatusForbidden, "you can only delete your own comments")
		return
	}
	database.DB.Exec(`DELETE FROM comments WHERE id = ?`, in.ID)
	auth.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
