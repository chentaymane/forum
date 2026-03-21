package handlers

import (
	"html/template"
	"net/http"
	"path/filepath"
	"strconv"

	"forum/auth"
	database "forum/db"
	"forum/forum"
)

var templateCache = make(map[string]*template.Template)

func getTemplate(name string) (*template.Template, error) {
	if tmpl, ok := templateCache[name]; ok {
		return tmpl, nil
	}

	// For each page, we parse the layout and the specific template
	tmpl, err := template.ParseFiles(
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
	MyComments  bool
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

// HomeHandler renders the home page with filtered posts.
func HomeHandler(w http.ResponseWriter, r *http.Request) {
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
	myPosts := r.URL.Query().Get("my_posts")
	myLiked := r.URL.Query().Get("my_liked_posts")
	myComments := r.URL.Query().Get("my_comments")
	pageStr := r.PathValue("page")
	if pageStr == "" {
		pageStr = r.URL.Query().Get("page")
	}

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
	commentedByUserID := 0
	if myComments == "1" {
		commentedByUserID = userID
	}

	pageSize := 12
	offset := (currentPage - 1) * pageSize

	posts, err := forum.GetPosts(catID, filterUserID, likedByUserID, commentedByUserID, pageSize, offset)
	if err != nil {
		http.Error(w, "Error fetching posts", http.StatusInternalServerError)
		return
	}

	totalPosts, err := forum.GetPostsCount(catID, filterUserID, likedByUserID, commentedByUserID)
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
		MyComments:  myComments == "1",
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
		http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, "base", data)
	if err != nil {
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
	}
}

// LoginPageHandler renders the login page.
func LoginPageHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "login", nil)
}

// RegisterPageHandler renders the registration page.
func RegisterPageHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "register", nil)
}

// CreatePostPageHandler renders the post creation page.
func CreatePostPageHandler(w http.ResponseWriter, r *http.Request) {
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
