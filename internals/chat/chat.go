package chat

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"rtforum/internals/auth"
	"rtforum/internals/database"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

var (
	mu      sync.Mutex
	clients = map[int][]*websocket.Conn{} // userID -> open connections
)

type wsMsg struct {
	Type     string `json:"type"`
	From     int    `json:"from,omitempty"`
	To       int    `json:"to,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	Content  string `json:"content,omitempty"`
	Date     string `json:"date,omitempty"`
	Users    []int  `json:"users,omitempty"`
}

// WsHandler upgrades the connection and handles real time chat.
func WsHandler(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r)
	if userID == 0 {
		auth.Error(w, http.StatusUnauthorized, "not logged in")
		return
	}
	var nickname string
	database.DB.QueryRow(`SELECT nickname FROM users WHERE id = ?`, userID).Scan(&nickname)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// Register connection and tell everyone who is online
	mu.Lock()
	clients[userID] = append(clients[userID], conn)
	mu.Unlock()
	broadcastOnline()

	// Read messages until the client disconnects
	for {
		var msg wsMsg
		if conn.ReadJSON(&msg) != nil {
			break
		}
		if msg.Type == "message" && msg.To > 0 && msg.To != userID && msg.Content != "" {
			deliver(userID, nickname, msg)
		}
	}

	// Unregister connection
	mu.Lock()
	conns := clients[userID]
	for i, c := range conns {
		if c == conn {
			clients[userID] = append(conns[:i], conns[i+1:]...)
			break
		}
	}
	if len(clients[userID]) == 0 {
		delete(clients, userID)
	}
	mu.Unlock()
	conn.Close()
	broadcastOnline()
}

// deliver stores the message and sends it to both users in real time.
func deliver(from int, nickname string, msg wsMsg) {
	date := time.Now().UTC().Format("2006-01-02 15:04")
	_, err := database.DB.Exec(`INSERT INTO messages (sender_id, receiver_id, content) VALUES (?, ?, ?)`,
		from, msg.To, msg.Content)
	if err != nil {
		return
	}
	out := wsMsg{Type: "message", From: from, To: msg.To, Nickname: nickname, Content: msg.Content, Date: date}
	sendTo(msg.To, out)
	sendTo(from, out)
}

// sendTo writes a message to every open connection of a user.
func sendTo(userID int, msg wsMsg) {
	mu.Lock()
	defer mu.Unlock()
	for _, c := range clients[userID] {
		c.WriteJSON(msg)
	}
}

// broadcastOnline sends the list of online user ids to everyone.
func broadcastOnline() {
	mu.Lock()
	defer mu.Unlock()
	ids := []int{}
	for id := range clients {
		ids = append(ids, id)
	}
	msg := wsMsg{Type: "online", Users: ids}
	for _, conns := range clients {
		for _, c := range conns {
			c.WriteJSON(msg)
		}
	}
}

type User struct {
	ID       int    `json:"id"`
	Nickname string `json:"nickname"`
}

// GetUsers returns all other users, ordered by last message then nickname.
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
			users = append(users, u)
		}
	}
	auth.JSON(w, http.StatusOK, users)
}

type Message struct {
	From     int    `json:"from"`
	Nickname string `json:"nickname"`
	Content  string `json:"content"`
	Date     string `json:"date"`
}

// GetMessages returns 10 messages of a conversation, oldest first.
// "offset" is the number of messages already loaded by the client.
func GetMessages(w http.ResponseWriter, r *http.Request) {
	me := auth.UserID(r)
	with, _ := strconv.Atoi(r.URL.Query().Get("with"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

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
	// Reverse so the oldest message comes first
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	auth.JSON(w, http.StatusOK, messages)
}
