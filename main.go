package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"rtforum/internals/auth"
	"rtforum/internals/chat"
	"rtforum/internals/database"
	"rtforum/internals/forum"
)

// serveSPA writes index.html with the given status code. Unknown routes still
// land on the single page; the frontend reads the URL and shows the error view.
func serveSPA(w http.ResponseWriter, status int) {
	page, err := os.ReadFile("./web/index.html")
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	w.Write(page)
}

// recoverMiddleware turns panics into a 500 response instead of a dropped
// connection: JSON for API calls, the single page for everything else.
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic on %s %s: %v", r.Method, r.URL.Path, err)
				if strings.HasPrefix(r.URL.Path, "/api/") {
					auth.Error(w, http.StatusInternalServerError, "internal server error")
				} else {
					serveSPA(w, http.StatusInternalServerError)
				}
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// staticHandler serves files from ./web: 404 (via the single page) for missing
// files and directories so nothing gets listed, 405 for non-GET methods.
func staticHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		serveSPA(w, http.StatusMethodNotAllowed)
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, "/static/")
	path := filepath.Join("./web", filepath.Clean("/"+rel))
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		serveSPA(w, http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, path)
}

func main() {
	// Initialize Database
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.DB.Close()

	// Static files + the single HTML page
	http.HandleFunc("/static/", staticHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			serveSPA(w, http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			serveSPA(w, http.StatusMethodNotAllowed)
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
	if err := http.ListenAndServe(":8080", recoverMiddleware(http.DefaultServeMux)); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
