package chat

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"forum/internals/auth"
	"forum/internals/database"

	"github.com/gorilla/websocket"
)

// upgrader turns an HTTP request into a websocket connection. Only pages
// served by this host may connect (blocks cross-site websocket hijacking).
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		return err == nil && u.Host == r.Host
	},
}

// clients holds every open connection per user (a user can be on several tabs).
// mu guards it because connections are added/removed from many goroutines.
var (
	mu      sync.Mutex
	clients = map[int][]*websocket.Conn{} // userID -> open connections
)

// wsMsg is the shape of every message exchanged over the websocket.
type wsMsg struct {
	Type     string `json:"type"` // "message" or "online"
	From     int    `json:"from,omitempty"`
	To       int    `json:"to,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	Content  string `json:"content,omitempty"`
	Date     string `json:"date,omitempty"`
	Users    []int  `json:"users,omitempty"` // online user ids, for "online" messages
}

// WsHandler upgrades the connection and handles real time chat for one client.
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
	// Drop the connection if the client sends an oversized frame.
	conn.SetReadLimit(2048)

	// Register the connection and tell everyone who is online.
	mu.Lock()
	clients[userID] = append(clients[userID], conn)
	mu.Unlock()
	broadcastOnline()

	// Read messages until the client disconnects.
	for {
		var msg wsMsg
		if conn.ReadJSON(&msg) != nil {
			break
		}
		if auth.UserID(r) == 0 {
			break
		}
		msg.Content = strings.TrimSpace(msg.Content)
		if msg.Type == "message" && msg.To > 0 && msg.To != userID &&
			msg.Content != "" && len(msg.Content) <= auth.MaxMessageLen {
			deliver(userID, nickname, msg)
		}
	}

	// Unregister this connection, dropping the user entry when it was the last.
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
