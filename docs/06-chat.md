# 6. Chat Package — `internals/chat`

## The WebSocket hub — `hub.go`

The real-time core. Two package-level variables:

```go
var mu sync.Mutex
var clients = map[int][]*websocket.Conn{}  // userID → open connections
```

It's a **slice** of connections per user because one user can have several
tabs open. Every access is guarded by the mutex, since each WebSocket
connection runs in its own goroutine.

### `WsHandler` — the lifecycle of one connection

1. **Authenticate**: check the session cookie with `auth.UserID` — the
   WebSocket handshake is a normal HTTP request, so cookies work.
2. **Upgrade**: `upgrader.Upgrade` switches the connection to the WebSocket
   protocol. The **`CheckOrigin`** function only accepts connections whose
   `Origin` header matches the server's host — this blocks other websites from
   silently opening a chat socket with your cookies (**cross-site WebSocket
   hijacking** protection).
3. `SetReadLimit(2048)` drops clients that send oversized frames.
4. **Register** the connection in `clients`, then call `broadcastOnline()` —
   sends `{type:"online", users:[ids]}` to *everyone*, so every sidebar
   updates its green/gray dots.
5. **Read loop** (`for { conn.ReadJSON(...) }`): for each incoming message it
   - re-checks the session is still valid (`auth.UserID(r)`),
   - trims the content,
   - validates: `to > 0`, not to yourself, non-empty, ≤ 500 chars,
   - and calls `deliver`.
6. **Cleanup**: when the loop breaks (tab closed, network error), the
   connection is removed from the map (dropping the user entry when it was the
   last one), and `broadcastOnline()` runs again so everyone sees the user go
   offline.

### `deliver(from, nickname, msg)`

Persists the message in the `messages` table **first**, then pushes it live
with `sendTo(receiver)` **and** `sendTo(sender)`. Sending to yourself is the
echo that makes the message appear in your own chat box (and in your other
open tabs).

### `sendTo(userID, msg)`

Loops over all of that user's open connections and `WriteJSON`s the message
to each.

### `broadcastOnline()`

Collects all user ids currently in the `clients` map and sends
`{type:"online", users:[...]}` to every open connection.

### `wsMsg` — the wire format

Every message over the socket has this shape:

```go
type wsMsg struct {
    Type     string // "message" or "online"
    From     int
    To       int
    Nickname string
    Content  string
    Date     string
    Users    []int  // online user ids, for "online" messages
}
```

## HTTP chat endpoints — `handlers.go`

### `GetUsers` — `GET /api/users`

Returns all users except yourself, with a `chatted` flag. The SQL computes the
timestamp of the **last message** between you and each user; sorting by that
DESC then by nickname gives:

1. people you talk to, most-recent-first (Discord style),
2. then everyone else alphabetically (case-insensitive).

The frontend uses `chatted` to split the sidebar into "Recent discussions" and
"Find friends".

### `GetMessages` — `GET /api/messages?with=<id>&offset=<n>`

Paginated history for the chat box:

- Fetches the newest 10 after the offset:
  `ORDER BY m.id DESC LIMIT 10 OFFSET ?`.
- Then **reverses the slice in Go** so the client receives them oldest-first,
  ready for display.
- `offset` = how many messages the client already has — used when you scroll
  to the top of the chat box to load older messages.
