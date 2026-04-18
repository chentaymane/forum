package forum

import (
	"database/sql"
	"strings"

	"forum/internals/database"
)

// Post represents a forum post.
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

// GetPosts retrieves posts with filtering and pagination.
func GetPosts(categoryID int, ofUserID int, userID int, likedByUserID int, commentedByUserID int, limit int, offset int) ([]Post, error) {
	var query strings.Builder
	var args []interface{}
	var where []string

	// BASE QUERY (NO ; AND NO GROUP BY HERE)
	query.WriteString(`
        SELECT 
            p.id,
            p.user_id,
            u.username,
            p.title,
            p.content,
            p.created_at,
            COALESCE(re.type, 0) AS reacted_to,
            COALESCE(likes.count, 0) AS likes,
            COALESCE(dislikes.count, 0) AS dislikes,
            GROUP_CONCAT(DISTINCT c.name) AS categories

        FROM posts p
        JOIN users u ON p.user_id = u.id

        LEFT JOIN reactions re
            ON p.id = re.post_id AND re.user_id = ?

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
    `)

	// always bind userID
	args = append(args, userID)

	// FILTER: category
	if categoryID > 0 {
		query.WriteString(" JOIN post_categories pc_filter ON p.id = pc_filter.post_id")
		where = append(where, "pc_filter.category_id = ?")
		args = append(args, categoryID)
	}

	// FILTER: liked
	if likedByUserID > 0 {
		where = append(where, `
            p.id IN (
                SELECT post_id 
                FROM reactions 
                WHERE user_id = ? AND (type = 1 OR type = -1) AND post_id IS NOT NULL
            )
        `)
		args = append(args, likedByUserID)
	}

	// FILTER: author
	if ofUserID > 0 {
		where = append(where, "p.user_id = ?")
		args = append(args, ofUserID)
	}

	// FILTER: commented
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

	//  GROUP BY (correct place)
	query.WriteString(" GROUP BY p.id")

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

	rows, err := database.DB.Query(query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		var categoriesStr sql.NullString

		if err := rows.Scan(
			&p.PostID,
			&p.UserID,
			&p.Username,
			&p.Title,
			&p.Content,
			&p.CreatedAt,
			&p.ReactedTo,
			&p.Likes,
			&p.Dislikes,
			&categoriesStr,
		); err != nil {
			return nil, err
		}

		p.CreatedAt = FormatDate(p.CreatedAt)

		if categoriesStr.Valid && categoriesStr.String != "" {
			p.Categories = strings.Split(categoriesStr.String, ",")
		} else {
			p.Categories = []string{}
		}

		p.Comments, _ = GetCommentsByPost(userID, p.PostID)
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
		query.WriteString(" JOIN reactions re ON p.id = re.post_id")
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
