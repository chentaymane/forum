package handlers

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"unicode/utf8"

	"forum/auth"
	"forum/database"
	"forum/forum"
)

var templateCache = make(map[string]*template.Template)

func getTemplate(name string) (*template.Template, error) {
	if tmpl, ok := templateCache[name]; ok {
		return tmpl, nil
	}

	// Custom template functions
	funcMap := template.FuncMap{
		"truncate": func(content string, maxLength int) string {
			if utf8.RuneCountInString(content) <= maxLength {
				return content
			}
			runes := []rune(content)
			return string(runes[:maxLength]) + "..."
		},
	}

	// For each page, we parse the layout and the specific template
	tmpl, err := template.New("base").Funcs(funcMap).ParseFiles(
		filepath.Join("templates", "layout.html"),
		filepath.Join("templates", name+".html"),
	)
	if err != nil {
		return nil, err
	}

	templateCache[name] = tmpl
	return tmpl, nil
}

// PageData represents the common data passed to templates.
type PageData struct {
	User        *User
	Posts       []forum.Post
	Categories  []Category
	Query       string
	CurrentPage int
	TotalPages  int
	CategoryID  int
	MyPosts     bool
	MyLiked     bool
	HasPrev     bool
	HasNext     bool
	PrevPage    int
	NextPage    int
}

type User struct {
	ID       int
	Username string
}

type Category struct {
	ID   int
	Name string
}

// ErrorData represents error page data.
type ErrorData struct {
	ErrorCode    int
	ErrorTitle   string
	ErrorMessage string
}

// RenderError renders an error page with the given status code and message.
func RenderError(w http.ResponseWriter, statusCode int, title string, message string) {
	tmpl, err := getTemplate("error")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)
	data := ErrorData{
		ErrorCode:    statusCode,
		ErrorTitle:   title,
		ErrorMessage: message,
	}

	err = tmpl.ExecuteTemplate(w, "base", data)
	if err != nil {
		// Can't write header again, just log
		log.Printf("Error rendering error page: %v", err)
	}
}

// HomeHandler renders the home page with filtered posts.
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		RenderError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "This endpoint only accepts GET requests.")
		return
	}
	userID, _ := auth.GetUserFromRequest(r)
	var currentUser *User
	if userID > 0 {
		var username string
		err := database.DB.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)
		if err == nil {
			currentUser = &User{ID: userID, Username: username}
		}
	}

	catIDStr := r.URL.Query().Get("category_id")
	pageStr := r.URL.Query().Get("page")
	myPosts := r.URL.Query().Get("my_posts")
	myLiked := r.URL.Query().Get("my_liked_posts")

	catID, _ := strconv.Atoi(catIDStr)
	currentPage, _ := strconv.Atoi(pageStr)
	if currentPage < 1 {
		currentPage = 1
	}

	filterUserID := 0
	if myPosts == "1" {
		filterUserID = userID
	}

	likedByUserID := 0
	if myLiked == "1" {
		likedByUserID = userID
	}

	pageSize := 10
	offset := (currentPage - 1) * pageSize

	posts, err := forum.GetPosts(catID, filterUserID, likedByUserID, pageSize, offset)
	if err != nil {
		http.Error(w, "Error fetching posts", http.StatusInternalServerError)
		return
	}

	totalPosts, err := forum.GetPostsCount(catID, filterUserID, likedByUserID)
	if err != nil {
		http.Error(w, "Error counting posts", http.StatusInternalServerError)
		return
	}

	totalPages := (totalPosts + pageSize - 1) / pageSize
	if totalPages == 0 && totalPosts == 0 {
		totalPages = 1
	}

	categories, _ := getCategories()

	data := PageData{
		User:        currentUser,
		Posts:       posts,
		Categories:  categories,
		CurrentPage: currentPage,
		TotalPages:  totalPages,
		CategoryID:  catID,
		MyPosts:     myPosts == "1",
		MyLiked:     myLiked == "1",
		HasPrev:     currentPage > 1,
		HasNext:     currentPage < totalPages,
		PrevPage:    currentPage - 1,
		NextPage:    currentPage + 1,
	}

	renderTemplate(w, "index", data)
}

func getCategories() ([]Category, error) {
	rows, err := database.DB.Query("SELECT id, name FROM categories")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

func renderTemplate(w http.ResponseWriter, tmplName string, data interface{}) {
	tmpl, err := getTemplate(tmplName)
	if err != nil {
		RenderError(w, http.StatusInternalServerError, "Error", "Failed to load page template.")
		return
	}

	err = tmpl.ExecuteTemplate(w, "base", data)
	if err != nil {
		RenderError(w, http.StatusInternalServerError, "Error", "Failed to render page.")
	}
}

// LoginPageHandler renders the login page.
func LoginPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		RenderError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "This endpoint only accepts GET requests.")
		return
	}
	renderTemplate(w, "login", nil)
}

// RegisterPageHandler renders the registration page.
func RegisterPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		RenderError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "This endpoint only accepts GET requests.")
		return
	}
	renderTemplate(w, "register", nil)
}

// CreatePostPageHandler renders the post creation page.
func CreatePostPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		RenderError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "This endpoint only accepts GET requests.")
		return
	}
	userID, _ := auth.GetUserFromRequest(r)
	if userID == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var username string
	database.DB.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)

	categories, _ := getCategories()
	data := PageData{
		User:       &User{ID: userID, Username: username},
		Categories: categories,
	}

	renderTemplate(w, "create_post", data)
}

// PostDetailPageHandler renders a single post's detail page.
func PostDetailPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		RenderError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "This endpoint only accepts GET requests.")
		return
	}

	// Extract post ID from URL path (/post/{id})
	postIDStr := r.URL.Path[len("/post/"):]
	if postIDStr == "" {
		RenderError(w, http.StatusBadRequest, "Bad Request", "Invalid post ID.")
		return
	}

	userID, _ := auth.GetUserFromRequest(r)
	var currentUser *User
	if userID > 0 {
		var username string
		err := database.DB.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)
		if err == nil {
			currentUser = &User{ID: userID, Username: username}
		}
	}

	postID, err := strconv.Atoi(postIDStr)
	if err != nil {
		RenderError(w, http.StatusBadRequest, "Bad Request", "Invalid post ID.")
		return
	}

	post, err := forum.GetPostByID(postID)
	if err != nil {
		RenderError(w, http.StatusNotFound, "Not Found", "The post you're looking for doesn't exist.")
		return
	}

	data := PageData{
		User:  currentUser,
		Posts: []forum.Post{*post},
	}

	renderTemplate(w, "post_detail", data)
}
