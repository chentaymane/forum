package forum

import (
	"net/http"
	"strconv"

	"forum/internals/auth"
	"forum/internals/database"
	"forum/internals/errors"
)

// ReactionsHandler handles likes and dislikes for posts and comments.
func ReactionsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserFromRequest(r)
	if err != nil {
		errors.RenderError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	postID := r.FormValue("post_id")
	commentID := r.FormValue("comment_id")
	likeType := r.FormValue("type")
	reactionType, _ := strconv.Atoi(likeType)

	if (postID == "" && commentID == "") || (likeType != "1" && likeType != "-1") {
		errors.RenderError(w, "Invalid parameters", http.StatusBadRequest)
		return
	}
	if errAdd := insertReaction(userID, postID, commentID, reactionType); errAdd != nil {
		errors.RenderError(w, "Error inserting error", http.StatusInternalServerError)
		return
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
		query = "SELECT type, COUNT(*) FROM reactions WHERE post_id = ? GROUP BY type"
		id = postID
	} else {
		query = "SELECT type, COUNT(*) FROM reactions WHERE comment_id = ? GROUP BY type"
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
		"SELECT type FROM reactions WHERE user_id = ? AND post_id = ?",
		userID, postID,
	).Scan(&t)
	return t
}

func insertReaction(userID int, postID, commentID string, reactionType int) error {
	_, err := database.DB.Exec(
		`INSERT INTO reactions (user_id, post_id, comment_id, type) VALUES (?, ?, ?, ?)`,
		userID,
		nilIfEmpty(postID),
		nilIfEmpty(commentID),
		reactionType,
	)
	return err
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
