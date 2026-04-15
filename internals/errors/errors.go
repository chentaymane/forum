package errors

import (
	"bytes"
	"html/template"
	"net/http"
	"path/filepath"
)

type ErrorData struct {
	Code    int
	Message string
}

func RenderError(w http.ResponseWriter, message string, code int) {
	tmpl, err := template.ParseFiles(
		filepath.Join("web", "templates", "error.html"),
	)
	if err != nil {
		http.Error(w, message, code)
		return
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, ErrorData{
		Code:    code,
		Message: message,
	})
	if err != nil {
		http.Error(w, "INTERNAL SERVER ERROR", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write(buf.Bytes())
}
