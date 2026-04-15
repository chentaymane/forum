package handlers

import (
	"bytes"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"text/template"

	"forum/internals/database"
	"forum/internals/errors"
)

var (
	templateCache = make(map[string]*template.Template)
	mu            sync.RWMutex
)

func getTemplate(name string) (*template.Template, error) {
	mu.RLock()
	defer mu.RUnlock()
	if tmpl, ok := templateCache[name]; ok {
		return tmpl, nil
	}

	// For each page, we parse the layout and the specific template
	tmpl, err := template.ParseFiles(
		filepath.Join("web", "templates", "layout.html"),
		filepath.Join("web", "templates", name+".html"),
	)
	if err != nil {
		return nil, err
	}

	templateCache[name] = tmpl
	return tmpl, nil
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
		log.Println(err)
		errors.RenderError(w, "Error loading template", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer

	// render into buffer instead of writing directly
	err = tmpl.ExecuteTemplate(&buf, "base", data)
	if err != nil {
		log.Println(err)
		errors.RenderError(w, "Error executing template", http.StatusInternalServerError)
		return
	}

	// write buffer to response
	_, _ = w.Write(buf.Bytes())
}

// GetUserData fetches user information by ID. Returns nil for anonymous users.
// Usage: user, err := GetUserData(userID); if err != nil { /* handle error */ }
func GetUserData(userID int) (*User, error) {
	if userID == 0 {
		return nil, nil // Anonymous user
	}

	var username string
	err := database.DB.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)
	if err != nil {
		return nil, err
	}

	return &User{ID: userID, Username: username}, nil
}
