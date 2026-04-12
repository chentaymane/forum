package handlers

import (
	"forum/internals/auth"
	"forum/internals/database"
	"forum/internals/errors"
	"forum/internals/forum"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// HomeHandler renders the home page with filtered posts.
func handleHome(w http.ResponseWriter, r *http.Request, page int) {
	userID, _ := auth.GetUserFromRequest(r)

	var currentUser *User
	if userID > 0 {
		var username string
		err := database.DB.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)
		if err == nil {
			currentUser = &User{ID: userID, Username: username}
		}
	}

	//  filters
	catIDStr := r.URL.Query().Get("category_id")
	myPosts := r.URL.Query().Get("my_posts")
	myLiked := r.URL.Query().Get("my_liked_posts")
	myComments := r.URL.Query().Get("my_comments")

	catID, _ := strconv.Atoi(catIDStr)

	//  use page passed from handler
	currentPage := page
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

	pageSize := 10
	offset := (currentPage - 1) * pageSize

	posts, err := forum.GetPosts(catID, filterUserID, userID, likedByUserID, commentedByUserID, pageSize, offset)
	if err != nil {
		log.Println(err)
		errors.RenderError(w, "Error fetching posts", http.StatusInternalServerError)
		return
	}

	totalPosts, err := forum.GetPostsCount(catID, filterUserID, likedByUserID, commentedByUserID)
	if err != nil {
		errors.RenderError(w, "Error counting posts", http.StatusInternalServerError)
		return
	}

	totalPages := (totalPosts + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	// FIX: must return after 404
	if currentPage > totalPages {
		errors.RenderError(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
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

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	//  "/"
	if path == "/" {
		handleHome(w, r, 1)
		return
	}

	//  "/posts/{page}"
	if strings.HasPrefix(path, "/page/") {
		parts := strings.Split(strings.Trim(path, "/"), "/")

		if len(parts) != 2 || parts[0] != "page" {
			errors.RenderError(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		page, err := strconv.Atoi(parts[1])
		if err != nil || page < 1 {
			errors.RenderError(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		handleHome(w, r, page)
		return
	}

	//  everything else
	errors.RenderError(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}
