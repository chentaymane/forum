package main

import (
	"fmt"
	"log"
	"net/http"

	"forum/auth"
	"forum/database"
	"forum/forum"
	"forum/handlers"
	"forum/middleware"
)

func main() {
	// Initialize Database
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Create a new ServeMux
	mux := http.NewServeMux()

	// Static Files
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	//  Public Page Handlers 
	mux.HandleFunc("/", handlers.HomeHandler)
	mux.HandleFunc("/posts/", handlers.HomeHandler)
	mux.HandleFunc("/login", handlers.LoginPageHandler)
	mux.HandleFunc("/register", handlers.RegisterPageHandler)

	// ── Protected Page Handlers 
	mux.HandleFunc("/create-post", middleware.AuthMiddleware(handlers.CreatePostPageHandler))

	//  Auth API (always public) 
	mux.HandleFunc("/auth/register", auth.RegisterHandler)
	mux.HandleFunc("/auth/login", auth.LoginHandler)
	mux.HandleFunc("/auth/logout", auth.LogoutHandler)

	//  Protected API Handlers 
	mux.HandleFunc("/api/posts", middleware.AuthMiddleware(forum.CreatePostHandler))
	mux.HandleFunc("/api/posts/delete", middleware.AuthMiddleware(forum.DeletePostHandler))
	mux.HandleFunc("/api/comments", middleware.AuthMiddleware(forum.CreateCommentHandler))
	mux.HandleFunc("/api/likes", middleware.AuthMiddleware(forum.LikeDislikeHandler))

	fmt.Println("Server started on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
