package forum

import (
	"fmt"
	"strings"
	"time"

	"forum/internals/database"
)

// Post represents a forum post.
type Post struct {
	ID          int
	UserID      int
	Username    string
	Title       string
	Content     string
	CreatedAt   string
	Categories  []string
	Likes       int
	Dislikes    int
	ReactedTo   int
	Comments    []Comment
	CommentsLen int
}

// CreatePostHandler handles post creation.

// GetPosts retrieves posts with filtering and pagination.
func GetPosts(categoryID int, ofUserID int, userID int, likedByUserID int, commentedByUserID int, limit int, offset int) ([]Post, error) {
	var query strings.Builder
	var args []interface{}
	var where []string

	// BASE QUERY
	query.WriteString(`
        SELECT p.id, 
            p.user_id, 
            u.username, 
            p.content, 
            p.created_at,
            COALESCE(re.type, 0) AS reacted_to
        FROM posts p
        JOIN users u ON p.user_id = u.id
        LEFT JOIN reactions re 
            ON p.id = re.post_id AND re.user_id = ?
    `)

	// always bind userID (0 if not logged in)
	args = append(args, userID)

	// FILTERS

	if categoryID > 0 {
		query.WriteString(" JOIN post_categories pc ON p.id = pc.post_id")
		where = append(where, "pc.category_id = ?")
		args = append(args, categoryID)
	}

	if likedByUserID > 0 {
		where = append(where, `
            p.id IN (
                SELECT post_id 
                FROM reactions 
                WHERE user_id = ? AND type = 1
            )
        `)
		args = append(args, likedByUserID)
	}

	if ofUserID > 0 {
		where = append(where, `
            p.id IN (
                SELECT post_id 
                FROM posts 
                WHERE user_id = ?
            )
        `)
		args = append(args, ofUserID)
	}

	if commentedByUserID > 0 {
		where = append(where, `
            p.id IN (
                SELECT post_id 
                FROM comments 
                WHERE user_id = ?
            )
        `)
		args = append(args, commentedByUserID)
	}

	// WHERE
	if len(where) > 0 {
		query.WriteString(" WHERE " + strings.Join(where, " AND "))
	}

	// ORDER
	query.WriteString(" ORDER BY p.created_at DESC")

	// PAGINATION
	if limit > 0 {
		query.WriteString(" LIMIT ?")
		args = append(args, limit)

		if offset > 0 {
			query.WriteString(" OFFSET ?")
			args = append(args, offset)
		}
	}

	// EXECUTE
	rows, err := database.DB.Query(query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post

	for rows.Next() {
		var p Post

		if err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.Username,
			&p.Content,
			&p.CreatedAt,
			&p.ReactedTo,
		); err != nil {
			return nil, err
		}
		fmt.Println(p.ReactedTo)
		p.CreatedAt = FormatDate(p.CreatedAt)
		p.Categories, _ = GetPostCategories(p.ID)
		p.Likes, p.Dislikes, _ = GetLikesCount(p.ID, 0)
		p.Comments, _ = GetCommentsByPost(p.ID)
		p.CommentsLen = len(p.Comments)
		if p.CommentsLen > 2 {
			p.Comments = p.Comments[:2]
		}

		posts = append(posts, p)
	}

	return posts, nil
}

// GetPostsCount returns the total number of posts matching the filters.
func GetPostsCount(categoryID int, userID int, likedByUserID int, commentedByUserID int) (int, error) {
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
		query.WriteString(" JOIN reactions ld ON p.id = re.post_id")
		where = append(where, "re.user_id = ? AND re.type = 1")
		args = append(args, likedByUserID)
	}

	if commentedByUserID > 0 {
		where = append(where, "p.id IN (SELECT post_id FROM comments WHERE user_id = ?)")
		args = append(args, commentedByUserID)
	}

	if len(where) > 0 {
		query.WriteString(" WHERE " + strings.Join(where, " AND "))
	}

	var count int
	err := database.DB.QueryRow(query.String(), args...).Scan(&count)
	return count, err
}

func GetPostCategories(postID int) ([]string, error) {
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

// Add this helper function
func FormatDate(raw string) string {
	// Try common SQLite datetime formats
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}
	for _, layout := range formats {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Format("2 Jan 2006 · 15:04")
		}
	}
	return raw // fallback: return as-is if parsing fails
}
