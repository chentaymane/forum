package forum

import (
	"net/http"
	"strconv"
	"strings"

	"forum/auth"
	"forum/database"
)

// Post represents a forum post.
type Post struct {
	ID         int
	UserID     int
	Username   string
	Content    string
	CreatedAt  string
	Categories []string
	Likes      int
	Dislikes   int
	Comments   []Comment
}

// CreatePostHandler handles post creation.
func CreatePostHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	content := r.FormValue("content")
	categoryIDsStr := r.PostForm["categories"] // Expected as multiples

	if content == "" {
		http.Error(w, "Content is required", http.StatusBadRequest)
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Insert post
	res, err := tx.Exec("INSERT INTO posts (user_id, content) VALUES (?, ?)", userID, content)
	if err != nil {
		http.Error(w, "Failed to create post", http.StatusInternalServerError)
		return
	}
	
	postID, _ := res.LastInsertId()

	// Associate categories
	for _, catID := range categoryIDsStr {
		_, err = tx.Exec("INSERT INTO post_categories (post_id, category_id) VALUES (?, ?)", postID, catID)
		if err != nil {
			http.Error(w, "Failed to associate categories", http.StatusInternalServerError)
			return
		}
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// GetPosts retrieves posts with filtering and pagination.
func GetPosts(categoryID int, userID int, likedByUserID int, limit int, offset int) ([]Post, error) {
	var query strings.Builder
	var args []interface{}

	query.WriteString(`
		SELECT p.id, p.user_id, u.username, p.content, p.created_at
		FROM posts p
		JOIN users u ON p.user_id = u.id
	`)

	where := []string{}

	if categoryID > 0 {
		query.WriteString(" JOIN post_categories pc ON p.id = pc.post_id")
		where = append(where, "pc.category_id = ?")
		args = append(args, categoryID)
	}

	if userID > 0 {
		where = append(where, "p.user_id = ?")
		args = append(args, userID)
	}

	if likedByUserID > 0 {
		query.WriteString(" JOIN likes_dislikes ld ON p.id = ld.post_id")
		where = append(where, "ld.user_id = ? AND ld.type = 1")
		args = append(args, likedByUserID)
	}

	if len(where) > 0 {
		query.WriteString(" WHERE " + strings.Join(where, " AND "))
	}

	query.WriteString(" ORDER BY p.created_at DESC")

	if limit > 0 {
		query.WriteString(" LIMIT ?")
		args = append(args, limit)
		if offset > 0 {
			query.WriteString(" OFFSET ?")
			args = append(args, offset)
		}
	}

	rows, err := database.DB.Query(query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.UserID, &p.Username, &p.Content, &p.CreatedAt); err != nil {
			return nil, err
		}
		
		// Load categories, likes, dislikes, and comments for each post
		p.Categories, _ = getPostCategories(p.ID)
		p.Likes, p.Dislikes, _ = GetLikesCount(p.ID, 0)
		p.Comments, _ = GetCommentsByPost(p.ID)
		posts = append(posts, p)
	}

	return posts, nil
}

// GetPostsCount returns the total number of posts matching the filters.
func GetPostsCount(categoryID int, userID int, likedByUserID int) (int, error) {
	var query strings.Builder
	var args []interface{}

	query.WriteString(`
		SELECT COUNT(DISTINCT p.id)
		FROM posts p
		JOIN users u ON p.user_id = u.id
	`)

	where := []string{}

	if categoryID > 0 {
		query.WriteString(" JOIN post_categories pc ON p.id = pc.post_id")
		where = append(where, "pc.category_id = ?")
		args = append(args, categoryID)
	}

	if userID > 0 {
		where = append(where, "p.user_id = ?")
		args = append(args, userID)
	}

	if likedByUserID > 0 {
		query.WriteString(" JOIN likes_dislikes ld ON p.id = ld.post_id")
		where = append(where, "ld.user_id = ? AND ld.type = 1")
		args = append(args, likedByUserID)
	}

	if len(where) > 0 {
		query.WriteString(" WHERE " + strings.Join(where, " AND "))
	}

	var count int
	err := database.DB.QueryRow(query.String(), args...).Scan(&count)
	return count, err
}

func getPostCategories(postID int) ([]string, error) {
	rows, err := database.DB.Query(`
		SELECT c.name FROM categories c
		JOIN post_categories pc ON c.id = pc.category_id
		WHERE pc.post_id = ?
	`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		categories = append(categories, name)
	}
	return categories, nil
}

// DeletePostHandler handles the deletion of a post.
func DeletePostHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	postIDStr := r.FormValue("post_id")
	postID, err := strconv.Atoi(postIDStr)
	if err != nil {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	// Verify ownership and delete in a transaction
	tx, err := database.DB.Begin()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Check if the user is the owner
	var ownerID int
	err = tx.QueryRow("SELECT user_id FROM posts WHERE id = ?", postID).Scan(&ownerID)
	if err != nil {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	if ownerID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Delete related comments, likes, and the post itself
	// SQLite Fk with ON DELETE CASCADE would be better, but we'll do it manually for safety if not configured.
	// Actually, let's just delete the post. If FKs are not cascading, we should delete them.
	_, err = tx.Exec("DELETE FROM comments WHERE post_id = ?", postID)
	if err != nil {
		http.Error(w, "Failed to delete comments", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec("DELETE FROM likes_dislikes WHERE post_id = ?", postID)
	if err != nil {
		http.Error(w, "Failed to delete reactions", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec("DELETE FROM post_categories WHERE post_id = ?", postID)
	if err != nil {
		http.Error(w, "Failed to delete post categories", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec("DELETE FROM posts WHERE id = ?", postID)
	if err != nil {
		http.Error(w, "Failed to delete post", http.StatusInternalServerError)
		return
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "Failed to commit deletion", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
