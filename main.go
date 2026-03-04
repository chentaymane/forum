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

	// Static Files
	fs := http.FileServer(http.Dir("static"))
	http.Handle("GET /static/", http.StripPrefix("/static/", fs))

	//  Public Page Handlers
	http.HandleFunc("GET /", handlers.HomeHandler)
	http.HandleFunc("GET /posts/{page}", handlers.HomeHandler)
	http.HandleFunc("GET /login", handlers.LoginPageHandler)
	http.HandleFunc("GET /register", handlers.RegisterPageHandler)

	//  Protected Page Handlers
	http.HandleFunc("GET /create-post", middleware.AuthMiddleware(handlers.CreatePostPageHandler))

	//  Auth API (always public)
	http.HandleFunc("POST /auth/register", auth.RegisterHandler)
	http.HandleFunc("POST /auth/login", auth.LoginHandler)
	http.HandleFunc("POST /auth/logout", auth.LogoutHandler)

	//  Protected API Handlers
	http.HandleFunc("POST /api/posts", middleware.AuthMiddleware(forum.CreatePostHandler))
	http.HandleFunc("POST /api/posts/delete", middleware.AuthMiddleware(forum.DeletePostHandler))
	http.HandleFunc("POST /api/comments", middleware.AuthMiddleware(forum.CreateCommentHandler))
	http.HandleFunc("POST /api/likes", middleware.AuthMiddleware(forum.LikeDislikeHandler))

	fmt.Println("Server started on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
