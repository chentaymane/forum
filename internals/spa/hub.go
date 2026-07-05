package spa

// ─── WebSocket Hub ──────────────────────────────────────────────────────────
// The Hub keeps track of which users currently have a live WebSocket
// connection (i.e. are "online").  It provides:
//
//   - SendToUser:     push a JSON event to a specific user (used for new
//                     private messages).
//   - BroadcastToAll: push a JSON event to every connected user (used for
//                     presence updates so everyone sees who's online).
//
// Each connected browser gets its own goroutine pair (readLoop / writeLoop).
// The server never reads meaningful messages from the client – it only reads
// to detect disconnection.  All real data flows server→client.

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"forum/internals/auth"

	"github.com/gorilla/websocket"
)

// upgrader promotes an HTTP connection to a WebSocket connection.
// CheckOrigin is set to allow any origin (safe for this project).
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ─── Hub ────────────────────────────────────────────────────────────────────

// Hub is the central registry of all active WebSocket clients.
type Hub struct {
	mu      sync.RWMutex
	clients map[int]*Client // userID → client
}

// Client wraps a single WebSocket connection for one user.
type Client struct {
	userID int
	conn   *websocket.Conn
	send   chan []byte // buffered channel of outbound messages
	hub    *Hub
}

// NewHub creates and returns an empty Hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[int]*Client),
	}
}

// Add registers a client.  If the user already has an open connection the old
// one is closed (only the most recent tab stays active).
func (h *Hub) Add(client *Client) {
	h.mu.Lock()
	if prev, ok := h.clients[client.userID]; ok {
		close(prev.send)
		_ = prev.conn.Close()
	}
	h.clients[client.userID] = client
	h.mu.Unlock()

	h.broadcastPresence()
}

// Remove unregisters a client (called when the connection drops).
func (h *Hub) Remove(client *Client) {
	h.mu.Lock()
	if current, ok := h.clients[client.userID]; ok && current == client {
		delete(h.clients, client.userID)
	}
	h.mu.Unlock()

	h.broadcastPresence()
}

// IsOnline reports whether userID currently has an active WebSocket.
func (h *Hub) IsOnline(userID int) bool {
	h.mu.RLock()
	_, ok := h.clients[userID]
	h.mu.RUnlock()
	return ok
}

// OnlineUserIDs returns a snapshot of all currently connected user IDs.
func (h *Hub) OnlineUserIDs() []int {
	h.mu.RLock()
	ids := make([]int, 0, len(h.clients))
	for id := range h.clients {
		ids = append(ids, id)
	}
	h.mu.RUnlock()
	return ids
}

// SendToUser sends a JSON payload to one user if they are connected.
func (h *Hub) SendToUser(userID int, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	h.mu.RLock()
	client := h.clients[userID]
	h.mu.RUnlock()

	if client == nil {
		return
	}

	select {
	case client.send <- data:
	default:
		// Client is too slow – drop the message to avoid blocking.
	}
}

// BroadcastToAll sends a JSON payload to every connected user.
func (h *Hub) BroadcastToAll(payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for _, c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		select {
		case client.send <- data:
		default:
		}
	}
}

// broadcastPresence tells every connected user who is currently online.
func (h *Hub) broadcastPresence() {
	h.BroadcastToAll(map[string]any{
		"type":            "presence",
		"online_user_ids": h.OnlineUserIDs(),
	})
}

// ─── HTTP → WebSocket ───────────────────────────────────────────────────────

// Serve upgrades an HTTP request to a WebSocket and runs the client loops
// until the connection closes.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserFromRequest(r)
	if err != nil || userID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &Client{
		userID: userID,
		conn:   conn,
		send:   make(chan []byte, 16),
		hub:    h,
	}

	h.Add(client)
	go client.writeLoop()

	// readLoop blocks until the connection drops
	client.readLoop()

	h.Remove(client)
	_ = conn.Close()
}

// ─── Client Goroutines ──────────────────────────────────────────────────────

// readLoop reads messages from the browser.  We don't send any meaningful
// data from the client, so this just detects disconnection.
func (c *Client) readLoop() {
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// writeLoop sends queued messages to the browser.  It also sends a periodic
// websocket.PingMessage to keep the connection alive through proxies.
func (c *Client) writeLoop() {
	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
