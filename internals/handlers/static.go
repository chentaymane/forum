package handlers

import (
	"forum/internals/errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func StaticFileHandlerHandler(w http.ResponseWriter, r *http.Request) {
	// strip /static/
	path := strings.TrimPrefix(r.URL.Path, "/static/")
	if path == "" {
		errors.RenderError(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// build full path
	fullPath := filepath.Join("web/static", path)

	// check if file exists
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		errors.RenderError(w, "File not found", http.StatusNotFound)
		return
	}

	// serve file
	http.ServeFile(w, r, fullPath)
}
