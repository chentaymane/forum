package errors

import (
	"html/template"
	"net/http"
	"path/filepath"
)

type ErrorData struct {
	User    interface{}
	Code    int
	Message string
}

func RenderError(w http.ResponseWriter, message string, code int) {
	w.WriteHeader(code)

	tmpl, err := template.ParseFiles(
		filepath.Join("web", "templates", "layout.html"),
		filepath.Join("web", "templates", "error.html"),
	)
	if err != nil {
		http.Error(w, message, code)
		return
	}

	err = tmpl.ExecuteTemplate(w, "base", ErrorData{User: nil, Code: code, Message: message})
	if err != nil {
		http.Error(w, message, code)
	}
}