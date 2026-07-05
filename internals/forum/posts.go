package forum

// ─── Post Data & Queries ────────────────────────────────────────────────────
// This file holds the Post struct and the two query functions that the SPA
// handler layer calls to fetch posts from the database.

import (
	"strings"

	"forum/internals/database"
)

// Post is the internal representation of a forum post.
// The SPA uses its own APIPost type for JSON – this struct is richer and
// includes reaction counts + a few preview comments.
type Post struct {
	PostID      int
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

// GetPostCategories fetches the category names attached to a post.
func GetPostCategories(postID int) []string {
	rows, err := database.DB.Query(`
		SELECT c.name FROM categories c
		JOIN post_categories pc ON c.id = pc.category_id
		WHERE pc.post_id = ?
	`, postID)
	if err != nil {
		return []string{}
	}
	defer rows.Close()
	var cats []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil {
			cats = append(cats, name)
		}
	}
	return cats
}

// GetPosts returns posts matching the provided filters, ordered by newest
// first.  It also attaches the first two comments (as a preview) and the
// current user's reaction state if they are logged in.
//
// Parameters:
//   - categoryID:      filter by category (0 = all)
//   - ofUserID:        only posts by this user (0 = any)
//   - userID:          the requesting user (for reaction state)
//   - likedByUserID:   only posts liked by this user
//   - commentedByUserID: only posts this user commented on
//   - limit:           max rows (0 = unlimited)
//   - offset:          pagination offset
func GetPosts(categoryID int, ofUserID int, userID int, likedByUserID int, commentedByUserID int, limit int, offset int) ([]Post, error) {
	var query strings.Builder
	var args []any
	var where []string

	query.WriteString(`
		SELECT p.id, p.user_id, COALESCE(u.nickname, u.username), p.title, p.content, p.created_at,
			COALESCE(re.type, 0) AS reacted_to,
			(SELECT COUNT(*) FROM reactions WHERE post_id = p.id AND type = 1) AS likes,
			(SELECT COUNT(*) FROM reactions WHERE post_id = p.id AND type = -1) AS dislikes
		FROM posts p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN reactions re ON p.id = re.post_id AND re.user_id = ?
	`)
	args = append(args, userID)

	if categoryID > 0 {
		where = append(where, "p.id IN (SELECT post_id FROM post_categories WHERE category_id = ?)")
		args = append(args, categoryID)
	}
	if likedByUserID > 0 {
		where = append(where, "p.id IN (SELECT post_id FROM reactions WHERE user_id = ?)")
		args = append(args, likedByUserID)
	}
	if ofUserID > 0 {
		where = append(where, "p.user_id = ?")
		args = append(args, ofUserID)
	}
	if commentedByUserID > 0 {
		where = append(where, "p.id IN (SELECT post_id FROM comments WHERE user_id = ?)")
		args = append(args, commentedByUserID)
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
		if err := rows.Scan(
			&p.PostID, &p.UserID, &p.Username, &p.Title, &p.Content,
			&p.CreatedAt, &p.ReactedTo, &p.Likes, &p.Dislikes,
		); err != nil {
			return nil, err
		}

		p.CreatedAt = FormatDate(p.CreatedAt)
		p.Categories = GetPostCategories(p.PostID)

		// Attach a preview of the first two comments
		p.Comments, _ = GetCommentsByPost(userID, p.PostID)
		p.CommentsLen = len(p.Comments)
		if p.CommentsLen > 2 {
			p.Comments = p.Comments[:2]
		}

		posts = append(posts, p)
	}

	return posts, nil
}

// GetPostsCount returns the total number of posts that match the filters.
// Used by the old paginated templates – the SPA currently ignores this.
func GetPostsCount(categoryID int, userID int, likedByUserID int, commentedByUserID int) (int, error) {
	var query strings.Builder
	var args []any
	var where []string

	query.WriteString("SELECT COUNT(*) FROM posts p")

	if categoryID > 0 {
		where = append(where, "p.id IN (SELECT post_id FROM post_categories WHERE category_id = ?)")
		args = append(args, categoryID)
	}
	if userID > 0 {
		where = append(where, "p.user_id = ?")
		args = append(args, userID)
	}
	if likedByUserID > 0 {
		where = append(where, "p.id IN (SELECT post_id FROM reactions WHERE user_id = ?)")
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
