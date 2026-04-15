package handlers

import (
	"net/http"
	"strconv"

	"forum/internals/auth"
	"forum/internals/database"
	"forum/internals/errors"
	"forum/internals/forum"
)

// CreatePostPageHandler renders the post creation page.
func CreatePostPageHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserFromRequest(r)
	if userID == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, _ := GetUserData(userID)

	categories, _ := getCategories()
	data := PageData{
		User:       user,
		Categories: categories,
	}

	renderTemplate(w, "create_post", data)
}

// PostDetails renders the full post detail page for /post/{post_id}.
func PostDetails(w http.ResponseWriter, r *http.Request) {
	// Extract post ID from URL pattern /post/{post_id}
	postIDStr := r.PathValue("post_id")
	postID, err := strconv.Atoi(postIDStr)
	if err != nil || postID <= 0 {
		errors.RenderError(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// Get current user (guests can view post details too)
	userID, _ := auth.GetUserFromRequest(r)
	currentUser, _ := GetUserData(userID)

	// Fetch the post with reaction state for the current user
	var post forum.Post
	err = database.DB.QueryRow(`
		SELECT p.id, p.user_id, u.username, p.title, p.content, p.created_at,
		       COALESCE(re.type, 0) AS reacted_to,
		       COALESCE(likes.count, 0) AS likes,
		       COALESCE(dislikes.count, 0) AS dislikes
		FROM posts p
		JOIN users u ON u.id = p.user_id
		LEFT JOIN reactions re ON p.id = re.post_id AND re.user_id = ?
		LEFT JOIN (
		    SELECT post_id, COUNT(*) as count
		    FROM reactions
		    WHERE type = 1 AND post_id IS NOT NULL
		    GROUP BY post_id
		) likes ON p.id = likes.post_id
		LEFT JOIN (
		    SELECT post_id, COUNT(*) as count
		    FROM reactions
		    WHERE type = -1 AND post_id IS NOT NULL
		    GROUP BY post_id
		) dislikes ON p.id = dislikes.post_id
		WHERE p.id = ?`,
		userID, postID,
	).Scan(
		&post.PostID,
		&post.UserID,
		&post.Username,
		&post.Title,
		&post.Content,
		&post.CreatedAt,
		&post.ReactedTo,
		&post.Likes,
		&post.Dislikes,
	)
	if err != nil {
		errors.RenderError(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// Format date consistently with the home feed
	post.CreatedAt = forum.FormatDate(post.CreatedAt)

	// Fetch categories for this post
	post.Categories, _ = forum.GetPostCategories(post.PostID)

	// Fetch ALL comments (no 2-comment limit like the home feed)
	post.Comments, _ = forum.GetCommentsByPost(userID, post.PostID)
	post.CommentsLen = len(post.Comments)

	// Fetch sidebar categories
	categories, _ := getCategories()

	data := PostDetailData{
		User:       currentUser,
		Post:       &post,
		Categories: categories,
	}

	renderTemplate(w, "post_details", data)
}
