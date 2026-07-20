package chat

import (
	"net/http"
	"strconv"

	"forum/internals/auth"
	"forum/internals/database"
)

// User is an entry in the chat contact list. Chatted tells the frontend
// whether a conversation exists, to split the list into two sections.
type User struct {
	ID       int    `json:"id"`
	Nickname string `json:"nickname"`
	Chatted  bool   `json:"chatted"`
}

// GetUsers returns all other users: the ones you chatted with first (most
// recent message first), then the rest alphabetically (case-insensitive).
func GetUsers(w http.ResponseWriter, r *http.Request) {
	me := auth.UserID(r)
	rows, err := database.DB.Query(`
		SELECT u.id, u.nickname,
			COALESCE((SELECT MAX(m.created_at) FROM messages m
				WHERE (m.sender_id = u.id AND m.receiver_id = ?)
				   OR (m.sender_id = ? AND m.receiver_id = u.id)), '') AS last
		FROM users u
		WHERE u.id != ?
		ORDER BY last DESC, u.nickname COLLATE NOCASE ASC`, me, me, me)
	if err != nil {
		auth.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		var u User
		var last string
		if rows.Scan(&u.ID, &u.Nickname, &last) == nil {
			u.Chatted = last != ""
			users = append(users, u)
		}
	}
	auth.JSON(w, http.StatusOK, users)
}

// Message is a single chat message returned to the client.
type Message struct {
	From     int    `json:"from"`
	Nickname string `json:"nickname"`
	Content  string `json:"content"`
	Date     string `json:"date"`
}

// GetMessages returns 10 messages of a conversation, oldest first.
// ?with=<userId> is the other person and ?offset=<n> is how many messages the
// client already loaded (used for "load more" scrolling).
func GetMessages(w http.ResponseWriter, r *http.Request) {
	me := auth.UserID(r)
	with, _ := strconv.Atoi(r.URL.Query().Get("with"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if with < 1 {
		auth.Error(w, http.StatusBadRequest, "invalid user")
		return
	}
	if offset < 0 {
		offset = 0
	}

	// Fetch the newest 10 after the offset...
	rows, err := database.DB.Query(`
		SELECT m.sender_id, u.nickname, m.content, substr(m.created_at, 1, 16)
		FROM messages m
		JOIN users u ON u.id = m.sender_id
		WHERE (m.sender_id = ? AND m.receiver_id = ?) OR (m.sender_id = ? AND m.receiver_id = ?)
		ORDER BY m.id DESC
		LIMIT 10 OFFSET ?`, me, with, with, me, offset)
	if err != nil {
		auth.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	messages := []Message{}
	for rows.Next() {
		var m Message
		if rows.Scan(&m.From, &m.Nickname, &m.Content, &m.Date) == nil {
			messages = append(messages, m)
		}
	}
	// ...then reverse so the oldest message comes first for display.
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	auth.JSON(w, http.StatusOK, messages)
}
