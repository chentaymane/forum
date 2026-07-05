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
//
// WHY THIS ARCHITECTURE?
// The Hub design (from gorilla/websocket examples) avoids per-connection
// goroutine leaks: Add/Remove are explicit, the send channel is buffered so
// slow clients don't block the Hub, and the writeLoop goroutine deals with
// the actual network I/O so readLoop stays simple.

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
	// TODO: In production, restrict CheckOrigin to your actual domain.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ─── Hub ────────────────────────────────────────────────────────────────────

// Hub is the central registry of all active WebSocket clients.
// Guards the clients map with a read-write mutex to allow concurrent
// access from multiple goroutines (one per connection).
type Hub struct {
	mu      sync.RWMutex
	clients map[int]*Client // userID → client (only the most recent tab)
}

// Client wraps a single WebSocket connection for one user.
// The send channel buffers up to 16 messages; if the buffer is full,
// new messages are dropped to avoid blocking the Hub.
type Client struct {
	userID int
	conn   *websocket.Conn
	send   chan []byte // buffered channel of outbound messages
	hub    *Hub
}

// NewHub creates and returns an empty Hub ready for use.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[int]*Client),
	}
}

// Add registers a client.  If the user already has an open connection the old
// one is closed (only the most recent tab stays active). This handles the
// common case where a user opens the forum in multiple tabs.
func (h *Hub) Add(client *Client) {
	h.mu.Lock()
	if prev, ok := h.clients[client.userID]; ok {
		close(prev.send)    // signal the old writeLoop to stop
		_ = prev.conn.Close() // close the old connection
	}
	h.clients[client.userID] = client
	h.mu.Unlock()

	h.broadcastPresence()
}

// Remove unregisters a client (called when the connection drops).
// Uses the pointer-identity check to avoid removing a newer connection
// for the same user that replaced this one.
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
// We copy the slice under the read lock so callers don't hold the lock
// while doing I/O.
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
// Uses the non-blocking send pattern: if the client's send buffer is full,
// the message is silently dropped. This prevents a slow client from
// blocking message delivery to other users.
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
// Used primarily for presence updates. Like SendToUser, this is
// non-blocking and drops messages for slow clients.
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
// The frontend uses this to show green/red dots next to user names.
func (h *Hub) broadcastPresence() {
	h.BroadcastToAll(map[string]any{
		"type":            "presence",
		"online_user_ids": h.OnlineUserIDs(),
	})
}

// ─── HTTP → WebSocket ───────────────────────────────────────────────────────

// Serve upgrades an HTTP request to a WebSocket and runs the client loops
// until the connection closes. This is the WebSocket handler registered
// in main.go at /ws.
//
// The client must have a valid session cookie — unauthenticated connections
// are rejected with 401.
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
// data from the client — the server only pushes events down. This loop
// simply detects when the connection drops (ReadMessage returns an error).
func (c *Client) readLoop() {
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// writeLoop sends queued messages to the browser.  It also sends a periodic
// ping to keep the connection alive through proxies and load balancers
// that might otherwise drop idle connections.
func (c *Client) writeLoop() {
	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// send channel was closed (user connected from another tab)
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			// Send a ping every 45 seconds. If the write fails,
			// the connection is dead — return to clean up.
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
