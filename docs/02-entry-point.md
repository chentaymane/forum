# 2. Entry Point — `main.go`

`main()` does four things in order.

## 1. Initialize the database

```go
if err := database.InitDB(); err != nil {
    log.Fatalf("Failed to initialize database: %v", err)
}
defer database.DB.Close()
```

Opens SQLite, creates tables, seeds categories (see [03-database.md](03-database.md)).
If it fails, the app stops immediately with `log.Fatalf`.

## 2. Static & page routes

### `staticHandler` — serves `/static/...`

Serves files from `./web` (CSS, JS). Three protections:

- `filepath.Join("./web", filepath.Clean("/"+rel))` cleans the path so nobody
  can request `/static/../main.go` (**path traversal protection**).
- Missing files and directories return 404 (via the single page), so nothing
  gets listed.
- Non-GET/HEAD methods return 405.

### `/` — the single page

Serves `./web/index.html`. **Any other URL** gets `serveSPA(w, 404)` — it still
sends index.html but with HTTP status 404; the frontend JS reads the URL and
shows the error view. This is the classic single-page-app pattern.

### `serveSPA(w, status)`

Writes `index.html` with the given status code. Used for 404, 405, and 500
responses on non-API paths — the user always lands on the app, which then shows
the right error view.

## 3. API routes with middleware

```go
// Auth API (MaxBody caps the size of every request body)
http.HandleFunc("/api/register", auth.MaxBody(auth.RegisterHandler))
http.HandleFunc("/api/login",    auth.MaxBody(auth.LoginHandler))
http.HandleFunc("/api/logout",   auth.LogoutHandler)
http.HandleFunc("/api/me",       auth.MeHandler)

// Forum API (protected)
http.HandleFunc("/api/categories", auth.Protect(forum.GetCategories))
http.HandleFunc("/api/posts",      auth.Protect(auth.MaxBody(forum.PostsHandler)))
http.HandleFunc("/api/comments",   auth.Protect(auth.MaxBody(forum.CommentsHandler)))
http.HandleFunc("/api/reactions",  auth.Protect(auth.MaxBody(forum.ReactionsHandler)))

// Chat API + Websocket (protected)
http.HandleFunc("/api/users",    auth.Protect(chat.GetUsers))
http.HandleFunc("/api/messages", auth.Protect(chat.GetMessages))
http.HandleFunc("/ws",           chat.WsHandler)
```

Two middleware wrappers:

- **`auth.MaxBody(...)`** — caps request bodies at 8 KB so oversized requests
  fail instead of being loaded into memory.
- **`auth.Protect(...)`** — rejects requests without a valid session cookie
  with `401 not logged in`, before the real handler ever runs.

## 4. Start the server

```go
http.ListenAndServe(":8080", recoverMiddleware(http.DefaultServeMux))
```

### `recoverMiddleware`

Wraps **everything**. If any handler panics, instead of crashing or dropping
the connection, it:

1. Logs the panic with the method and path.
2. Returns a 500 response — JSON (`{"error": "internal server error"}`) for
   `/api/...` paths, the HTML single page for everything else.
