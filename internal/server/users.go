package server

import (
	"net/http"
)

func HelloWorld(w http.ResponseWriter, r *http.Request) {
	parsedPages.ExecuteTemplate(w, "hello", nil)
}

func register(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		parsedPages.ExecuteTemplate(w, "register", nil)
	}
}

func login(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		parsedPages.ExecuteTemplate(w, "login", nil)
	}
}
