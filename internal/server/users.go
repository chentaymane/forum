package server

import (
	"net/http"
)

func HelloWorld(w http.ResponseWriter, r *http.Request) {
	parsedPages.ExecuteTemplate(w, "hello", nil)
}
