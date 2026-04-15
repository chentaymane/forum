package forum

import (
	"fmt"
	"net/http"
	"strconv"

	"forum/internals/auth"
	"forum/internals/database"
	"forum/internals/errors"
)

var ErrInvalidTarget = fmt.Errorf("invalid target")

// ReactionsHandler handles likes and dislikes for posts and comments.
func ReactionsHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserFromRequest(r)

	postID := r.FormValue("post_id")
	commentID := r.FormValue("comment_id")
	likeType := r.FormValue("type")
	// fmt.Println(likeType)
	reactionType, err := strconv.Atoi(likeType)

	if (postID == "" && commentID == "") || (postID != "" && commentID != "") || (likeType != "1" && likeType != "-1") || err != nil {
		errors.RenderError(w, "Invalid parameters", http.StatusBadRequest)
		return
	}
	if errAdd := insertReaction(userID, postID, commentID, reactionType); errAdd != nil {
		if errAdd == ErrInvalidTarget {
			errors.RenderError(w, "Error inserting error: Invalid Post/Comment Id", http.StatusBadRequest)
			return
		}
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

func insertReaction(userID int, postID, commentID string, reactionType int) error {
	// check if post or comment exist first
	var exists bool
	var err error

	if postID != "" && commentID == "" {
		err = database.DB.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM posts WHERE id = ?)`,
			postID,
		).Scan(&exists)
	} else if postID == "" && commentID != "" {
		err = database.DB.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM comments WHERE id = ?)`,
			commentID,
		).Scan(&exists)
	}
	if err != nil || !exists {
		fmt.Println("aloo")
		return ErrInvalidTarget
	}
	_, err = database.DB.Exec(
		`INSERT INTO reactions (user_id, post_id, comment_id, type)
		 SELECT ?, ?, ?, ?`,
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
