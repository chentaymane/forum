package handlers

import (
	"forum/internals/database"
	"forum/internals/errors"
	"net/http"
	"path/filepath"
	"sync"
	"text/template"
)

var templateCache = make(map[string]*template.Template)
var mu sync.RWMutex

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
		errors.RenderError(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, "base", data)
	if err != nil {
		errors.RenderError(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
