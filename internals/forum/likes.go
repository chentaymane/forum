package forum

import (
	"net/http"

	"forum/internals/auth"
	"forum/internals/database"
)

// LikeDislikeHandler handles likes and dislikes for posts and comments.
func LikeDislikeHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	postID := r.FormValue("post_id")
	commentID := r.FormValue("comment_id")
	likeType := r.FormValue("type")

	if (postID == "" && commentID == "") || (likeType != "1" && likeType != "-1") {
		http.Error(w, "Invalid parameters", http.StatusBadRequest)
		return
	}

	var targetID string
	var checkQuery, deleteQuery, insertQuery string

	if postID != "" {
		targetID = postID
		checkQuery = "SELECT COUNT(*) FROM likes_dislikes WHERE user_id = ? AND post_id = ? AND type = ?"
		deleteQuery = "DELETE FROM likes_dislikes WHERE user_id = ? AND post_id = ?"
		insertQuery = "INSERT INTO likes_dislikes (user_id, post_id, type) VALUES (?, ?, ?)"
	} else {
		targetID = commentID
		checkQuery = "SELECT COUNT(*) FROM likes_dislikes WHERE user_id = ? AND comment_id = ? AND type = ?"
		deleteQuery = "DELETE FROM likes_dislikes WHERE user_id = ? AND comment_id = ?"
		insertQuery = "INSERT INTO likes_dislikes (user_id, comment_id, type) VALUES (?, ?, ?)"
	}

	// Check if the user already reacted with the SAME type → toggle off
	var existing int
	database.DB.QueryRow(checkQuery, userID, targetID, likeType).Scan(&existing)

	// Always remove the previous reaction first
	database.DB.Exec(deleteQuery, userID, targetID)

	// Only insert if it was NOT the same reaction (toggle off if same)
	if existing == 0 {
		_, err = database.DB.Exec(insertQuery, userID, targetID, likeType)
		if err != nil {
			http.Error(w, "Failed to process like/dislike", http.StatusInternalServerError)
			return
		}
	}

	// Redirect back to the referring page (or home as fallback)
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

// GetLikesCount returns the number of likes and dislikes for a given target.
func GetLikesCount(postID, commentID int) (likes, dislikes int, err error) {
	var query string
	var id int
	if postID > 0 {
		query = "SELECT type, COUNT(*) FROM likes_dislikes WHERE post_id = ? GROUP BY type"
		id = postID
	} else {
		query = "SELECT type, COUNT(*) FROM likes_dislikes WHERE comment_id = ? GROUP BY type"
		id = commentID
	}

	rows, err := database.DB.Query(query, id)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var lType, count int
		if err := rows.Scan(&lType, &count); err != nil {
			return 0, 0, err
		}
		switch lType {
		case 1:
			likes = count
		case -1:
			dislikes = count
		}
	}

	return likes, dislikes, nil
}

// GetUserReaction returns whether the user has liked (1) or disliked (-1) a post, or 0 for neither.
func GetUserReaction(userID, postID int) int {
	var t int
	database.DB.QueryRow(
		"SELECT type FROM likes_dislikes WHERE user_id = ? AND post_id = ?",
		userID, postID,
	).Scan(&t)
	return t
}
