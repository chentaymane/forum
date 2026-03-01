package forum

import (
	"net/http"

	"forum/auth"
	"forum/database"
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
	likeType := r.FormValue("type") // "1" for like, "-1" for dislike

	if (postID == "" && commentID == "") || (likeType != "1" && likeType != "-1") {
		http.Error(w, "Target ID and A valid type are required", http.StatusBadRequest)
		return
	}

	// Determine if we are liking a post or a comment
	var query string
	var targetID string
	if postID != "" {
		deletePrevReaction(userID, postID, "")
		query = "INSERT INTO likes_dislikes (user_id, post_id, type) VALUES (?, ?, ?)"
		targetID = postID
	} else if commentID != "" {
		deletePrevReaction(userID, "", commentID)
		query = "INSERT INTO likes_dislikes (user_id, comment_id, type) VALUES (?, ?, ?)"
		targetID = commentID
	} else {
		http.Error(w, "Invalid target ID", http.StatusBadRequest)
		return
	}

	_, err = database.DB.Exec(query, userID, targetID, likeType)
	if err != nil {
		http.Error(w, "Failed to process like/dislike", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func deletePrevReaction(userID int, postID, commentID string) {
	if postID != "" {
		database.DB.Exec("DELETE FROM likes_dislikes WHERE user_id = ? AND post_id = ?", userID, postID)
	} else if commentID != "" {
		database.DB.Exec("DELETE FROM likes_dislikes WHERE user_id = ? AND comment_id = ?", userID, commentID)
	}
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
