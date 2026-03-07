package forum

import (
	"encoding/json"
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

	// Get updated counts
	var likes, dislikes int
	if postID != "" {
		id, _ := strconv.Atoi(postID)
		likes, dislikes, _ = GetLikesCount(id, 0)
	} else {
		id, _ := strconv.Atoi(commentID)
		likes, dislikes, _ = GetLikesCount(0, id)
	}

	// Return JSON for AJAX requests
	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
		r.Header.Get("Accept") == "application/json" {

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{
			"likes":    likes,
			"dislikes": dislikes,
		})
		return
	}
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
