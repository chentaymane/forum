package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

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

	user, _ := getUserData(userID)

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
	currentUser, _ := getUserData(userID)

	// Fetch the post with reaction state for the current user
	var post forum.Post
	var categoriesStr sql.NullString
	err = database.DB.QueryRow(`
		SELECT p.id, p.user_id, u.username, p.title, p.content, p.created_at,
		       	COALESCE(re.type, 0) AS reacted_to,
		       	COALESCE(likes.count, 0) AS likes,
		       	COALESCE(dislikes.count, 0) AS dislikes,
            	GROUP_CONCAT(DISTINCT c.name) AS categories
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
		LEFT JOIN post_categories pc_all ON p.id = pc_all.post_id
        LEFT JOIN categories c ON pc_all.category_id = c.id
		WHERE p.id = ?
		`,
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
		&categoriesStr,
	)
	if err != nil {
		errors.RenderError(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// Format date consistently with the home feed
	post.CreatedAt = forum.FormatDate(post.CreatedAt)

	// Fetch categories for this post
	if categoriesStr.Valid && categoriesStr.String != "" {
		post.Categories = strings.Split(categoriesStr.String, ",")
	} else {
		post.Categories = []string{}
	}

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
