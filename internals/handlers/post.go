package handlers

import (
	"forum/internals/auth"
	"forum/internals/database"
	"forum/internals/forum"
	"net/http"
	"strconv"
)

// CreatePostPageHandler renders the post creation page.
func CreatePostPageHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserFromRequest(r)
	if userID == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var username string
	database.DB.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)

	categories, _ := getCategories()
	data := PageData{
		User:       &User{ID: userID, Username: username},
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
		http.NotFound(w, r)
		return
	}

	// Get current user (guests can view post details too)
	userID, _ := auth.GetUserFromRequest(r)
	var currentUser *User
	if userID != 0 {
		var username string
		database.DB.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)
		currentUser = &User{ID: userID, Username: username}
	}

	// Fetch the post with reaction state for the current user
	var post forum.Post
	err = database.DB.QueryRow(`
		SELECT p.id, p.user_id, u.username, p.title, p.content, p.created_at,
		       COALESCE(re.type, 0) AS reacted_to
		FROM posts p
		JOIN users u ON u.id = p.user_id
		LEFT JOIN reactions re ON p.id = re.post_id AND re.user_id = ?
		WHERE p.id = ?`,
		userID, postID,
	).Scan(
		&post.ID,
		&post.UserID,
		&post.Username,
		&post.Title,
		&post.Content,
		&post.CreatedAt,
		&post.ReactedTo,
	)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Format date consistently with the home feed
	post.CreatedAt = forum.FormatDate(post.CreatedAt)

	// Fetch like / dislike counts
	post.Likes, post.Dislikes, _ = forum.GetLikesCount(post.ID, 0)

	// Fetch categories for this post
	post.Categories, _ = forum.GetPostCategories(post.ID)

	// Fetch ALL comments (no 2-comment limit like the home feed)
	post.Comments, _ = forum.GetCommentsByPost(post.ID)
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
