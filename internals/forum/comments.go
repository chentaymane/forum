package forum

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"rtforum/internals/auth"
	"rtforum/internals/database"
)

// CommentsHandler lists a post's comments (GET) or creates one (POST).
func CommentsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getComments(w, r)
	case http.MethodPost:
		createComment(w, r)
	default:
		auth.Error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// getComments returns the comments of a post (?post_id=<id>), oldest first,
// each with its like/dislike counts and the current user's reaction.
func getComments(w http.ResponseWriter, r *http.Request) {
	postID, _ := strconv.Atoi(r.URL.Query().Get("post_id"))
	rows, err := database.DB.Query(`
		SELECT c.id, u.nickname, c.content, substr(c.created_at, 1, 16),
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
		if rows.Scan(&c.ID, &c.Nickname, &c.Content, &c.Date, &c.Likes, &c.Dislikes, &c.ReactedTo) == nil {
			comments = append(comments, c)
		}
	}
	auth.JSON(w, http.StatusOK, comments)
}

// createComment inserts a comment on a post.
func createComment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PostID  int    `json:"postId"`
		Content string `json:"content"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
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
