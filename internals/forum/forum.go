package forum

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"rtforum/internals/auth"
	"rtforum/internals/database"
)

type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Post struct {
	ID         int    `json:"id"`
	Nickname   string `json:"nickname"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	Categories string `json:"categories"`
	Date       string `json:"date"`
	Likes      int    `json:"likes"`
	Dislikes   int    `json:"dislikes"`
	ReactedTo  int    `json:"reactedTo"` // 1, -1 or 0 for the logged in user
	Comments   int    `json:"comments"`
}

type Comment struct {
	ID        int    `json:"id"`
	Nickname  string `json:"nickname"`
	Content   string `json:"content"`
	Date      string `json:"date"`
	Likes     int    `json:"likes"`
	Dislikes  int    `json:"dislikes"`
	ReactedTo int    `json:"reactedTo"`
}

// GetCategories returns all categories.
func GetCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`SELECT id, name FROM categories ORDER BY id`)
	if err != nil {
		auth.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	categories := []Category{}
	for rows.Next() {
		var c Category
		if rows.Scan(&c.ID, &c.Name) == nil {
			categories = append(categories, c)
		}
	}
	auth.JSON(w, http.StatusOK, categories)
}

// PostsHandler lists posts (GET) or creates a post (POST).
func PostsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getPosts(w, r)
	case http.MethodPost:
		createPost(w, r)
	default:
		auth.Error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// getPosts returns the feed, newest first, with optional filters:
// ?category=<id>, ?mine=1, ?commented=1, ?liked=1
func getPosts(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r)
	q := r.URL.Query()

	query := `
		SELECT p.id, u.nickname, p.title, p.content, substr(p.created_at, 1, 16),
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
	query += " ORDER BY p.id DESC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		auth.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	posts := []Post{}
	for rows.Next() {
		var p Post
		if rows.Scan(&p.ID, &p.Nickname, &p.Title, &p.Content, &p.Date,
			&p.Categories, &p.Likes, &p.Dislikes, &p.ReactedTo, &p.Comments) == nil {
			posts = append(posts, p)
		}
	}
	auth.JSON(w, http.StatusOK, posts)
}

// createPost inserts a new post with its categories.
func createPost(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title      string `json:"title"`
		Content    string `json:"content"`
		Categories []int  `json:"categories"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		auth.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Content = strings.TrimSpace(in.Content)
	if in.Title == "" || in.Content == "" {
		auth.Error(w, http.StatusBadRequest, "title and content are required")
		return
	}

	res, err := database.DB.Exec(`INSERT INTO posts (user_id, title, content) VALUES (?, ?, ?)`,
		auth.UserID(r), in.Title, in.Content)
	if err != nil {
		auth.Error(w, http.StatusInternalServerError, "failed to create post")
		return
	}
	postID, _ := res.LastInsertId()
	for _, catID := range in.Categories {
		database.DB.Exec(`INSERT OR IGNORE INTO post_categories (post_id, category_id) VALUES (?, ?)`, postID, catID)
	}
	auth.JSON(w, http.StatusOK, map[string]any{"id": postID})
}

// CommentsHandler lists a post's comments (GET) or creates one (POST).
func CommentsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getComments(w, r)
	case http.MethodPost:
		createComment(w, r)
	default:
		auth.Error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// getComments returns the comments of a post, oldest first.
func getComments(w http.ResponseWriter, r *http.Request) {
	postID, _ := strconv.Atoi(r.URL.Query().Get("post_id"))
	rows, err := database.DB.Query(`
		SELECT c.id, u.nickname, c.content, substr(c.created_at, 1, 16),
			(SELECT COUNT(*) FROM reactions WHERE comment_id = c.id AND type = 1),
			(SELECT COUNT(*) FROM reactions WHERE comment_id = c.id AND type = -1),
			COALESCE((SELECT type FROM reactions WHERE comment_id = c.id AND user_id = ?), 0)
		FROM comments c
		JOIN users u ON u.id = c.user_id
		WHERE c.post_id = ?
		ORDER BY c.id`, auth.UserID(r), postID)
	if err != nil {
		auth.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	comments := []Comment{}
	for rows.Next() {
		var c Comment
		if rows.Scan(&c.ID, &c.Nickname, &c.Content, &c.Date, &c.Likes, &c.Dislikes, &c.ReactedTo) == nil {
			comments = append(comments, c)
		}
	}
	auth.JSON(w, http.StatusOK, comments)
}

// createComment inserts a comment on a post.
func createComment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PostID  int    `json:"postId"`
		Content string `json:"content"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		auth.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	in.Content = strings.TrimSpace(in.Content)
	if in.PostID < 1 || in.Content == "" {
		auth.Error(w, http.StatusBadRequest, "comment is empty")
		return
	}

	_, err := database.DB.Exec(`INSERT INTO comments (post_id, user_id, content) VALUES (?, ?, ?)`,
		in.PostID, auth.UserID(r), in.Content)
	if err != nil {
		auth.Error(w, http.StatusInternalServerError, "failed to create comment")
		return
	}
	auth.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ReactionsHandler likes/dislikes a post or a comment.
// Same type again = toggle off, opposite type = replace.
func ReactionsHandler(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PostID    int `json:"postId"`
		CommentID int `json:"commentId"`
		Type      int `json:"type"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil ||
		(in.Type != 1 && in.Type != -1) ||
		(in.PostID > 0) == (in.CommentID > 0) {
		auth.Error(w, http.StatusBadRequest, "invalid parameters")
		return
	}
	userID := auth.UserID(r)

	// column and target of the reaction
	column, target := "post_id", in.PostID
	if in.CommentID > 0 {
		column, target = "comment_id", in.CommentID
	}

	var existing int
	err := database.DB.QueryRow(
		`SELECT type FROM reactions WHERE user_id = ? AND `+column+` = ?`,
		userID, target).Scan(&existing)
	if err == nil {
		database.DB.Exec(`DELETE FROM reactions WHERE user_id = ? AND `+column+` = ?`, userID, target)
	}
	if err != nil || existing != in.Type { // insert unless it was toggled off
		if _, err := database.DB.Exec(
			`INSERT INTO reactions (user_id, `+column+`, type) VALUES (?, ?, ?)`,
			userID, target, in.Type); err != nil {
			auth.Error(w, http.StatusBadRequest, "invalid post/comment id")
			return
		}
	}

	// return the fresh counts so the client updates without refreshing
	var out struct {
		Likes     int `json:"likes"`
		Dislikes  int `json:"dislikes"`
		ReactedTo int `json:"reactedTo"`
	}
	database.DB.QueryRow(`SELECT
		(SELECT COUNT(*) FROM reactions WHERE `+column+` = ? AND type = 1),
		(SELECT COUNT(*) FROM reactions WHERE `+column+` = ? AND type = -1),
		COALESCE((SELECT type FROM reactions WHERE `+column+` = ? AND user_id = ?), 0)`,
		target, target, target, userID).Scan(&out.Likes, &out.Dislikes, &out.ReactedTo)
	auth.JSON(w, http.StatusOK, out)
}
