package main

// ═══════════════════════════════════════════════════════════════════════════
//   Forum – Entry Point
// ═══════════════════════════════════════════════════════════════════════════
//
// This file starts the HTTP server, initialises the database, and registers
// all routes.  The server speaks JSON to the SPA frontend.
//
// ─── How it works ─────────────────────────────────────────────────────────
// 1. Open/initialize the SQLite database.
// 2. Mount routes – note that Go 1.24's net/http supports path parameters
//    like {post_id} natively (no external router needed).
// 3. Every API handler returns JSON.  The SPA shell (index.html) is served
//    at the root path.
// 4. WebSocket connections are upgraded at /ws.

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
	// ── Database ──────────────────────────────────────────────────────
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.DB.Close()

	// ── SPA Application ───────────────────────────────────────────────
	// The App struct owns all API handlers plus the WebSocket hub.
	app := spa.NewApp()

	// ── Routes ────────────────────────────────────────────────────────

	// Static assets (CSS, JS).  The SPA loads these from /static/*.
	http.HandleFunc("/static/", staticFileHandler)

	// SPA shell – serves the single HTML file.  Every page load hits this.
	http.HandleFunc("/", middleware.Method("GET", app.ShellHandler))

	// Authentication
	http.HandleFunc("/api/me", middleware.Method("GET", app.MeHandler))
	http.HandleFunc("/api/register", middleware.Method("POST", app.RegisterHandler))
	http.HandleFunc("/api/login", middleware.Method("POST", app.LoginHandler))
	http.HandleFunc("/api/logout", middleware.Method("POST", app.LogoutHandler))

	// Categories, Posts & Reactions
	http.HandleFunc("/api/reactions", middleware.Method("POST", app.ReactionHandler))

	// Categories & Posts
	http.HandleFunc("/api/categories", middleware.Method("GET", app.CategoriesHandler))
	http.HandleFunc("/api/posts", app.PostsHandler)                          // GET (list) + POST (create)
	http.HandleFunc("/api/posts/{post_id}", middleware.Method("GET", app.PostHandler))
	http.HandleFunc("/api/posts/{post_id}/comments", middleware.Method("POST", app.CommentsHandler))

	// Private Messaging
	http.HandleFunc("/api/chat/contacts", middleware.Method("GET", app.ChatContactsHandler))
	http.HandleFunc("/api/chat/messages", middleware.Method("GET", app.ChatMessagesHandler))
	http.HandleFunc("/api/messages", middleware.Method("POST", app.MessageSendHandler))

	// Real‑time via WebSocket
	http.HandleFunc("/ws", middleware.Method("GET", app.WebSocketHandler))

	// ── Start Server ──────────────────────────────────────────────────
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
// It strips the /static/ prefix and serves the matching file.
func staticFileHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/static/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	fullPath := filepath.Join("web/static", path)

	// Basic security: prevent directory traversal
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
