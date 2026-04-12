package main

import (
	"fmt"
	"log"
	"net/http"

	"forum/internals/auth"
	"forum/internals/database"
	"forum/internals/forum"
	"forum/internals/handlers"
	"forum/internals/middleware"
)

func main() {
	// Initialize Database
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Create a new ServeMux

	// Static Files
	fs := http.FileServer(http.Dir("web/static"))
	http.Handle("GET /static/", http.StripPrefix("/static/", fs))

	//  Public Page Handlers
	http.HandleFunc("/", middleware.Method("GET", handlers.HomeHandler))
	http.HandleFunc("/page/{page}", middleware.Method("GET", handlers.HomeHandler))
	http.HandleFunc("/post/{post_id}", middleware.Method("GET", handlers.PostDetails))
	http.HandleFunc("/login", middleware.Method("GET", middleware.Guest(handlers.LoginPageHandler)))
	http.HandleFunc("/register", middleware.Method("GET", middleware.Guest(handlers.RegisterPageHandler)))

	//  Protected Page Handlers
	http.HandleFunc("/create-post", middleware.Method("GET", middleware.AuthMiddleware(handlers.CreatePostPageHandler)))

	//  Auth API (always public)
	http.HandleFunc("/auth/register", middleware.Method("POST", middleware.Guest(auth.RegisterHandler)))
	http.HandleFunc("/auth/login", middleware.Method("POST", middleware.Guest(auth.LoginHandler)))
	http.HandleFunc("/auth/logout", middleware.Method("POST", middleware.AuthMiddleware(auth.LogoutHandler)))

	//  Protected API Handlers
	http.HandleFunc("/api/posts/create", middleware.Method("POST", middleware.AuthMiddleware(forum.CreatePost)))
	http.HandleFunc("/api/posts/delete", middleware.Method("POST", middleware.AuthMiddleware(forum.DeletePost)))
	http.HandleFunc("/api/comments", middleware.Method("POST", middleware.AuthMiddleware(forum.CreateCommentHandler)))
	http.HandleFunc("/api/likes", middleware.Method("POST", middleware.AuthMiddleware(forum.ReactionsHandler)))

	fmt.Println("Server started on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
