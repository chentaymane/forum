package forum

import (
	"encoding/json"
	"net/http"

	"forum/internals/auth"
	"forum/internals/database"
)

// ReactionsHandler likes/dislikes a post or a comment.
// Sending the same type again toggles it off; sending the opposite replaces it.
func ReactionsHandler(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PostID    int `json:"postId"`
		CommentID int `json:"commentId"`
		Type      int `json:"type"`
	}
	// Type must be like (1) or dislike (-1), and exactly one of postId /
	// commentId must be set (the "!=" acts as an exclusive-or here).
	if json.NewDecoder(r.Body).Decode(&in) != nil ||
		(in.Type != 1 && in.Type != -1) ||
		(in.PostID > 0) == (in.CommentID > 0) {
		auth.Error(w, http.StatusBadRequest, "invalid parameters")
		return
	}
	userID := auth.UserID(r)

	// Pick the column and id we are reacting to.
	column, target := "post_id", in.PostID
	if in.CommentID > 0 {
		column, target = "comment_id", in.CommentID
	}

	// Remove any existing reaction from this user on this target first.
	var existing int
	err := database.DB.QueryRow(
		`SELECT type FROM reactions WHERE user_id = ? AND `+column+` = ?`,
		userID, target).Scan(&existing)
	if err == nil {
		database.DB.Exec(`DELETE FROM reactions WHERE user_id = ? AND `+column+` = ?`, userID, target)
	}
	// Insert the new reaction unless the user just toggled the same one off.
	if err != nil || existing != in.Type {
		if _, err := database.DB.Exec(
			`INSERT INTO reactions (user_id, `+column+`, type) VALUES (?, ?, ?)`,
			userID, target, in.Type); err != nil {
			auth.Error(w, http.StatusBadRequest, "invalid post/comment id")
			return
		}
	}

	// Return the fresh counts so the client updates without refreshing.
	var out struct {
		Likes     int `json:"likes"`
		Dislikes  int `json:"dislikes"`
		ReactedTo int `json:"reactedTo"`
	}
	database.DB.QueryRow(`SELECT
		(SELECT COUNT(*) FROM reactions WHERE `+column+` = ? AND type = 1),
		(SELECT COUNT(*) FROM reactions WHERE `+column+` = ? AND type = -1),
		COALESCE((SELECT type FROM reactions WHERE `+column+` = ? AND user_id = ?), 0)`,
		target, target, target, userID).Scan(&out.Likes, &out.Dislikes, &out.ReactedTo)
	auth.JSON(w, http.StatusOK, out)
}
