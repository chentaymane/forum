package forum

// Category is a topic a post can belong to (e.g. "Technology").
type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Post is a forum thread shown in the feed.
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

// Comment is a reply attached to a post.
type Comment struct {
	ID        int    `json:"id"`
	Nickname  string `json:"nickname"`
	Content   string `json:"content"`
	Date      string `json:"date"`
	Likes     int    `json:"likes"`
	Dislikes  int    `json:"dislikes"`
	ReactedTo int    `json:"reactedTo"`
}
