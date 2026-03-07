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

type loggingHandler struct {
	handler http.Handler
}

func (lh *loggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Wrap response writer to detect if handler writes
	wrapped := &statusWriter{ResponseWriter: w, statusCode: http.StatusOK}
	lh.handler.ServeHTTP(wrapped, r)

	// If we got 404, show custom error page
	if wrapped.statusCode == http.StatusNotFound {
		handlers.RenderError(w, http.StatusNotFound, "Not Found", "The page you're looking for doesn't exist.")
	}
}

type statusWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (sw *statusWriter) WriteHeader(code int) {
	if !sw.written {
		sw.statusCode = code
		sw.written = true
		sw.ResponseWriter.WriteHeader(code)
	}
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if !sw.written {
		sw.statusCode = http.StatusOK
		sw.written = true
	}
	return sw.ResponseWriter.Write(b)
}

func main() {
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Static files
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Page Handlers
	http.HandleFunc("/", handlers.HomeHandler)
	http.HandleFunc("/post/", handlers.PostDetailPageHandler)
	http.HandleFunc("/login", handlers.LoginPageHandler)
	http.HandleFunc("/register", handlers.RegisterPageHandler)
	http.HandleFunc("/create-post", handlers.CreatePostPageHandler)

	// API Handlers
	http.HandleFunc("/auth/register", auth.RegisterHandler)
	http.HandleFunc("/auth/login", auth.LoginHandler)
	http.HandleFunc("/auth/logout", auth.LogoutHandler)
	http.HandleFunc("/api/posts", forum.CreatePostHandler)
	http.HandleFunc("/api/posts/delete", forum.DeletePostHandler)
	http.HandleFunc("/api/comments", forum.CreateCommentHandler)
	http.HandleFunc("/api/likes", forum.LikeDislikeHandler)

	// Wrap with logging/404 handler
	handler := &loggingHandler{handler: http.DefaultServeMux}

	fmt.Println("Server started on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		fmt.Println("Error starting server:", err)
	}
}
