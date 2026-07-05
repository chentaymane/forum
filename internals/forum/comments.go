package forum

// ─── Comment Model ─────────────────────────────────────────────────────────
// Comment represents a single reply on a post.  Like Post, this internal
// struct carries reaction data; the SPA handler converts it to APIComment
// for JSON serialization.
//
// The separation between forum.Comment and spa.APIComment is intentional:
// the forum package deals with raw database rows (with side data like
// reaction state), while the spa package decides exactly what to expose.

import "forum/internals/database"

// Comment is the database-level representation including reaction data.
type Comment struct {
	CommentID  int
	PostID     int
	UserID     int
	Username   string
	Content    string
	CreatedAt  string
	ComL       int // likes count
	ComD       int // dislikes count
	ReactedToC int // current user's reaction (1, -1, or 0)
}

// GetCommentsByPost returns all comments for a given post, ordered oldest
// first (chronological order, same as any forum).  Includes the reaction
// state for the requesting user so they can see their own like/dislike.
//
// WHY A SEPARATE FUNCTION?
// Comments are loaded lazily — the feed only shows the preview; the full
// comment list is fetched when the user opens a post. This keeps the feed
// response lightweight.
func GetCommentsByPost(userID, postID int) ([]Comment, error) {
	query := `
		SELECT c.id, c.post_id, c.user_id, COALESCE(u.nickname, u.username), c.content, c.created_at,
			COALESCE(r.type, 0) AS reacted_to,
			(SELECT COUNT(*) FROM reactions WHERE comment_id = c.id AND type = 1) AS likes,
			(SELECT COUNT(*) FROM reactions WHERE comment_id = c.id AND type = -1) AS dislikes
		FROM comments c
		JOIN users u ON c.user_id = u.id
		LEFT JOIN reactions r ON r.comment_id = c.id AND r.user_id = ?
		WHERE c.post_id = ?
		ORDER BY c.created_at ASC
	`
	rows, err := database.DB.Query(query, userID, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(
			&c.CommentID, &c.PostID, &c.UserID, &c.Username,
			&c.Content, &c.CreatedAt, &c.ReactedToC, &c.ComL, &c.ComD,
		); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}

	return comments, nil
}
