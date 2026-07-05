package main

// ═══════════════════════════════════════════════════════════════════════════════
//  FORUM — Entry Point
// ═══════════════════════════════════════════════════════════════════════════════
//
//  This is where the server starts. We:
//  1. Open the SQLite database (forum.db).
//  2. Create the SPA application (holds all API handlers + WebSocket hub).
//  3. Register HTTP routes (each one maps to a handler function).
//  4. Start the HTTP server on port 8080 (or $PORT).
//
//  ARCHITECTURE:
//  The frontend is a single-page app (SPA). The server:
//    - Serves index.html at the root path "/"
//    - Serves static files (CSS, JS) from /static/
//    - Provides a JSON API at /api/* for the SPA to call
//    - Provides a WebSocket endpoint at /ws for real-time messaging
//
//  SECURITY:
//  - All SQL queries use parameterized placeholders (?) — no SQL injection.
//  - All user input is validated before storage (see app.go validators).
//  - Session cookies are HttpOnly + SameSite=Lax (prevents XSS + CSRF).
//  - Passwords are hashed with bcrypt before storage.
//  - The frontend uses escapeHtml() for all user text in the DOM.

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"forum/internals/database"
	"forum/internals/middleware"
	"forum/internals/spa"
)

func main() {
	// ── 1. Initialize the database ──────────────────────────────────────
	// Opens forum.db (or creates it), runs the schema, seeds categories.
	// The DB connection is stored in database.DB and used by all handlers.
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.DB.Close()

	// ── 2. Create the SPA Application ───────────────────────────────────
	// The App struct owns ALL HTTP handlers plus the WebSocket hub.
	// We create one App and register its methods as route handlers.
	app := spa.NewApp()

	// ── 3. Register Routes ──────────────────────────────────────────────
	//
	// Each route specifies:
	//   - PATH (e.g., "/api/login")
	//   - METHOD (enforced by middleware.Method)
	//   - HANDLER (a method on the App struct)
	//
	// Go 1.22+ supports path parameters like {post_id} natively in
	// http.HandleFunc — no external router library needed.

	// Static assets (CSS, JS). The browser loads these from <link> and
	// <script> tags in index.html. We serve them from the web/static/
	// directory with a prefix-stripping handler.
	http.HandleFunc("/static/", staticFileHandler)

	// SPA shell — serves index.html for every GET request to "/".
	// The JavaScript in app.js handles all client-side routing.
	http.HandleFunc("/", middleware.Method("GET", app.ShellHandler))

	// ── Authentication routes ───────────────────────────────────────────
	// /api/me       → GET    → returns the logged-in user (or 401)
	// /api/register  → POST   → creates a new account + sets session cookie
	// /api/login     → POST   → authenticates + sets session cookie
	// /api/logout    → POST   → destroys the session + clears cookie
	http.HandleFunc("/api/me", middleware.Method("GET", app.MeHandler))
	http.HandleFunc("/api/register", middleware.Method("POST", app.RegisterHandler))
	http.HandleFunc("/api/login", middleware.Method("POST", app.LoginHandler))
	http.HandleFunc("/api/logout", middleware.Method("POST", app.LogoutHandler))

	// ── Content routes ──────────────────────────────────────────────────
	// /api/categories         → GET  → list all categories
	// /api/posts              → GET  → list posts (with optional filter)
	// /api/posts              → POST → create a new post
	// /api/posts/{post_id}    → GET  → get single post with comments
	// /api/posts/{post_id}/comments → POST → add a comment
	// /api/reactions          → POST → like/dislike a post or comment
	http.HandleFunc("/api/categories", middleware.Method("GET", app.CategoriesHandler))
	http.HandleFunc("/api/posts", app.PostsHandler) // dispatches GET or POST
	http.HandleFunc("/api/posts/{post_id}", middleware.Method("GET", app.PostHandler))
	http.HandleFunc("/api/posts/{post_id}/comments", middleware.Method("POST", app.CommentsHandler))
	http.HandleFunc("/api/reactions", middleware.Method("POST", app.ReactionHandler))

	// ── Private messaging routes ────────────────────────────────────────
	// /api/chat/contacts → GET  → list all users (for the chat sidebar)
	// /api/chat/messages  → GET  → get conversation with a specific user
	// /api/messages       → POST → send a private message
	http.HandleFunc("/api/chat/contacts", middleware.Method("GET", app.ChatContactsHandler))
	http.HandleFunc("/api/chat/messages", middleware.Method("GET", app.ChatMessagesHandler))
	http.HandleFunc("/api/messages", middleware.Method("POST", app.MessageSendHandler))

	// ── Real-time WebSocket ─────────────────────────────────────────────
	// The WebSocket is used for:
	//   1. Pushing new private messages to the recipient in real-time.
	//   2. Broadcasting presence updates (who's online).
	// The server authenticates the WebSocket using the session cookie.
	http.HandleFunc("/ws", middleware.Method("GET", app.WebSocketHandler))

	// ── 4. Start the HTTP Server ────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("✓ Server running at http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

// staticFileHandler serves files from the web/static/ directory.
//
// HOW IT WORKS:
// The browser requests /static/style.css. We strip the "/static/" prefix
// to get "style.css", then look for it in web/static/style.css.
//
// SECURITY:
// We check that the resolved path starts with "web/static" to prevent
// directory traversal attacks (e.g. "/static/../../etc/passwd").
func staticFileHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/static/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	fullPath := filepath.Join("web/static", path)

	// Prevent directory traversal: the resolved path must start with
	// the allowed directory prefix. This stops attacks using ".." in
	// the URL to access files outside web/static/.
	if !strings.HasPrefix(fullPath, "web/static") {
		http.NotFound(w, r)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, fullPath)
}
