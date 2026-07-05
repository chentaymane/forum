package spa

// ─── API Response Types ─────────────────────────────────────────────────────
// These structs define the JSON shape that the Go backend sends to the
// JavaScript frontend.  Every field has a json tag so the encoder uses the
// correct lowercase names that the JS code expects.
//
// Note: "omitempty" fields are left out of the response when they are zero,
// which saves a few bytes on the wire.

// APIUser is the public profile returned after login/registration.
type APIUser struct {
	ID        int    `json:"id"`
	Email     string `json:"email,omitempty"`
	Nickname  string `json:"nickname"`
	Age       int    `json:"age,omitempty"`
	Gender    string `json:"gender,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Online    bool   `json:"online,omitempty"`
}

// APICategory maps the categories table to JSON.
type APICategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// APIComment is the comment shape used in post detail views.
type APIComment struct {
	ID        int    `json:"id"`
	PostID    int    `json:"post_id"`
	UserID    int    `json:"user_id"`
	Nickname  string `json:"nickname"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// APIPost is the shape used for both the feed list and the detail view.
// When included in a feed the Comments field is omitted; the frontend loads
// it separately when the user clicks a post.
type APIPost struct {
	ID           int          `json:"id"`
	UserID       int          `json:"user_id"`
	Nickname     string       `json:"nickname"`
	Title        string       `json:"title"`
	Content      string       `json:"content"`
	CreatedAt    string       `json:"created_at"`
	Categories   []string     `json:"categories"`
	Likes        int          `json:"likes"`
	Dislikes     int          `json:"dislikes"`
	ReactedTo    int          `json:"reacted_to"` // 1 = liked, -1 = disliked, 0 = none
	CommentCount int          `json:"comment_count"`
	Comments     []APIComment `json:"comments,omitempty"`
}

// ChatContact is the user card shown in the always‑visible sidebar.
// Ordered by last message time (most recent first, users with no messages
// sorted alphabetically).
type ChatContact struct {
	ID            int    `json:"id"`
	Nickname      string `json:"nickname"`
	FirstName     string `json:"first_name,omitempty"`
	LastName      string `json:"last_name,omitempty"`
	Online        bool   `json:"online"`
	LastMessageAt string `json:"last_message_at,omitempty"`
}

// ChatMessage is a single private message.
type ChatMessage struct {
	ID           int    `json:"id"`
	SenderID     int    `json:"sender_id"`
	ReceiverID   int    `json:"receiver_id"`
	SenderName   string `json:"sender_name"`
	ReceiverName string `json:"receiver_name"`
	Content      string `json:"content"`
	CreatedAt    string `json:"created_at"`
}
