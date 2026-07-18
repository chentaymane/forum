package forum

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"rtforum/internals/auth"
	"rtforum/internals/database"
)

// PostsHandler lists posts (GET), creates a post (POST), or deletes one (POST with action=delete).
func PostsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getPosts(w, r)
	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			auth.Error(w, http.StatusBadRequest, "invalid request")
			return
		}
		var probe struct {
			Action string `json:"action"`
		}
		if json.Unmarshal(body, &probe) == nil && probe.Action == "delete" {
			deletePost(w, r, body)
			return
		}
		createPostFromBody(w, r, body)
	default:
		auth.Error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// getPosts returns the feed 10 posts at a time, newest first, with optional
// filters: ?category=<id>, ?mine=1, ?commented=1, ?liked=1.
// ?offset=<n> is how many posts the client already loaded.
func getPosts(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r)
	q := r.URL.Query()

	// The base query also computes, per post: its category names, its like and
	// dislike counts, how the current user reacted, and its comment count.
	query := `
		SELECT p.id, p.user_id, u.nickname, p.title, p.content, substr(p.created_at, 1, 16),
			COALESCE((SELECT GROUP_CONCAT(c.name, ', ') FROM categories c
				JOIN post_categories pc ON pc.category_id = c.id
				WHERE pc.post_id = p.id), ''),
			(SELECT COUNT(*) FROM reactions WHERE post_id = p.id AND type = 1),
			(SELECT COUNT(*) FROM reactions WHERE post_id = p.id AND type = -1),
			COALESCE((SELECT type FROM reactions WHERE post_id = p.id AND user_id = ?), 0),
			(SELECT COUNT(*) FROM comments WHERE post_id = p.id)
		FROM posts p
		JOIN users u ON u.id = p.user_id`
	args := []any{userID}
	var where []string

	// Build the WHERE clause from whichever filters are present.
	if q.Get("category") != "" {
		where = append(where, "p.id IN (SELECT post_id FROM post_categories WHERE category_id = ?)")
		args = append(args, q.Get("category"))
	}
	if q.Get("mine") == "1" {
		where = append(where, "p.user_id = ?")
		args = append(args, userID)
	}
	if q.Get("commented") == "1" {
		where = append(where, "p.id IN (SELECT post_id FROM comments WHERE user_id = ?)")
		args = append(args, userID)
	}
	if q.Get("liked") == "1" {
		where = append(where, "p.id IN (SELECT post_id FROM reactions WHERE user_id = ? AND type = 1)")
		args = append(args, userID)
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	if offset < 0 {
		offset = 0
	}
	query += " ORDER BY p.id DESC LIMIT 10 OFFSET ?"
	args = append(args, offset)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		auth.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	posts := []Post{}
	for rows.Next() {
		var p Post
		if rows.Scan(&p.ID, &p.UserID, &p.Nickname, &p.Title, &p.Content, &p.Date,
			&p.Categories, &p.Likes, &p.Dislikes, &p.ReactedTo, &p.Comments) == nil {
			posts = append(posts, p)
		}
	}
	auth.JSON(w, http.StatusOK, posts)
}

// createPostFromBody inserts a new post together with its categories.
func createPostFromBody(w http.ResponseWriter, r *http.Request, body []byte) {
	var in struct {
		Title      string `json:"title"`
		Content    string `json:"content"`
		Categories []int  `json:"categories"`
	}
	if json.Unmarshal(body, &in) != nil {
		auth.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Content = strings.TrimSpace(in.Content)
	if in.Title == "" || in.Content == "" {
		auth.Error(w, http.StatusBadRequest, "title and content are required")
		return
	}
	if len(in.Title) > auth.MaxTitleLen || len(in.Content) > auth.MaxContentLen {
		auth.Error(w, http.StatusBadRequest, "title or content too long")
		return
	}

	res, err := database.DB.Exec(`INSERT INTO posts (user_id, title, content) VALUES (?, ?, ?)`,
		auth.UserID(r), in.Title, in.Content)
	if err != nil {
		auth.Error(w, http.StatusInternalServerError, "failed to create post")
		return
	}
	// Link the post to each selected category.
	postID, _ := res.LastInsertId()
	var generalID int
	database.DB.QueryRow(`SELECT id FROM categories WHERE name = 'General'`).Scan(&generalID)

	valid := map[int]bool{}
	rows, err := database.DB.Query(`SELECT id FROM categories`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int
			if rows.Scan(&id) == nil {
				valid[id] = true
			}
		}
	}

	seen := map[int]bool{}
	for _, catID := range in.Categories {
		if valid[catID] && !seen[catID] {
			database.DB.Exec(`INSERT OR IGNORE INTO post_categories (post_id, category_id) VALUES (?, ?)`, postID, catID)
			seen[catID] = true
		}
	}
	if len(seen) == 0 && generalID > 0 {
		database.DB.Exec(`INSERT OR IGNORE INTO post_categories (post_id, category_id) VALUES (?, ?)`, postID, generalID)
	}
	auth.JSON(w, http.StatusOK, map[string]any{"id": postID})
}

// deletePost removes a post if the requester is its owner.
func deletePost(w http.ResponseWriter, r *http.Request, body []byte) {
	var in struct {
		ID int `json:"id"`
	}
	if json.Unmarshal(body, &in) != nil || in.ID < 1 {
		auth.Error(w, http.StatusBadRequest, "invalid post id")
		return
	}
	userID := auth.UserID(r)
	var ownerID int
	if err := database.DB.QueryRow(`SELECT user_id FROM posts WHERE id = ?`, in.ID).Scan(&ownerID); err != nil {
		auth.Error(w, http.StatusNotFound, "post not found")
		return
	}
	if ownerID != userID {
		auth.Error(w, http.StatusForbidden, "you can only delete your own posts")
		return
	}
	database.DB.Exec(`DELETE FROM posts WHERE id = ?`, in.ID)
	auth.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
