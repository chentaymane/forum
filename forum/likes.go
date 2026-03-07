package forum

import (
	"net/http"
	"strconv"

	"forum/auth"
	"forum/database"
)

// LikeDislikeHandler handles likes and dislikes for posts and comments.
func LikeDislikeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := auth.GetUserFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	postID := r.FormValue("post_id")
	commentID := r.FormValue("comment_id")
	likeType := r.FormValue("type") // "1" or "-1"

	if (postID == "" && commentID == "") || (likeType != "1" && likeType != "-1") {
		http.Error(w, "Invalid reaction request", http.StatusBadRequest)
		return
	}

	reqType, _ := strconv.Atoi(likeType)

	// Single insert: triggers handle toggle logic
	_, err = database.DB.Exec(
		`INSERT INTO likes_dislikes (user_id, post_id, comment_id, type) VALUES (?, ?, ?, ?)`,
		userID,
		nilIfEmpty(postID),
		nilIfEmpty(commentID),
		reqType,
	)
	if err != nil {
		http.Error(w, "Failed to process reaction", http.StatusInternalServerError)
		return
	}

	// Redirect back to the post page with fragment (normal form submit, no JS)
	redirectPostID := postID
	fragment := "#postreaction"
	if redirectPostID == "" {
		var pid int
		if err := database.DB.QueryRow("SELECT post_id FROM comments WHERE id = ?", commentID).Scan(&pid); err == nil {
			redirectPostID = strconv.Itoa(pid)
			fragment = "#comment" + commentID
		}
	}
	if redirectPostID != "" {
		http.Redirect(w, r, "/post/"+redirectPostID+fragment, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Helper: return nil for empty string so SQLite inserts NULL
func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
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
