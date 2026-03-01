package main

import (
	"fmt"
	"log"
	"net/http"

	"forum/auth"
	"forum/database"
	"forum/forum"
	"forum/handlers"
)

func main() {
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	mux := http.NewServeMux()

	// Static files
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	// Page Handlers
	mux.HandleFunc("GET /", handlers.HomeHandler)
	mux.HandleFunc("GET /posts/{page}", handlers.HomeHandler)
	mux.HandleFunc("GET /login", handlers.LoginPageHandler)
	mux.HandleFunc("GET /register", handlers.RegisterPageHandler)
	mux.HandleFunc("GET /create-post", handlers.CreatePostPageHandler)

	// API Handlers
	mux.HandleFunc("POST /auth/register", auth.RegisterHandler)
	mux.HandleFunc("POST /auth/login", auth.LoginHandler)
	mux.HandleFunc("POST /auth/logout", auth.LogoutHandler)
	mux.HandleFunc("POST /api/posts/", forum.CreatePostHandler)
	mux.HandleFunc("POST /api/posts/delete", forum.DeletePostHandler)
	mux.HandleFunc("POST /api/comments", forum.CreateCommentHandler)
	mux.HandleFunc("POST /api/likes", forum.LikeDislikeHandler)

	fmt.Println("Server started on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		fmt.Println("Error starting server:", err)
	}
}
