package main

import (
	"fmt"
	"log"
	"net/http"

	"rtforum/internals/auth"
	"rtforum/internals/chat"
	"rtforum/internals/database"
	"rtforum/internals/forum"
)

func main() {
	// Initialize Database
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.DB.Close()

	// Static files + the single HTML page
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./web"))))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "./web/index.html")
	})

	// Auth API (MaxBody caps the size of every request body)
	http.HandleFunc("/api/register", auth.MaxBody(auth.RegisterHandler))
	http.HandleFunc("/api/login", auth.MaxBody(auth.LoginHandler))
	http.HandleFunc("/api/logout", auth.LogoutHandler)
	http.HandleFunc("/api/me", auth.MeHandler)

	// Forum API (protected)
	http.HandleFunc("/api/categories", auth.Protect(forum.GetCategories))
	http.HandleFunc("/api/posts", auth.Protect(auth.MaxBody(forum.PostsHandler)))
	http.HandleFunc("/api/comments", auth.Protect(auth.MaxBody(forum.CommentsHandler)))
	http.HandleFunc("/api/reactions", auth.Protect(auth.MaxBody(forum.ReactionsHandler)))

	// Chat API + Websocket (protected)
	http.HandleFunc("/api/users", auth.Protect(chat.GetUsers))
	http.HandleFunc("/api/messages", auth.Protect(chat.GetMessages))
	http.HandleFunc("/ws", chat.WsHandler)

	fmt.Println("Server started on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
