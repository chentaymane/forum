package handlers

import "net/http"

// LoginPageHandler renders the login page.
func LoginPageHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "login", PageData{})
}

// RegisterPageHandler renders the registration page.
func RegisterPageHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "register", PageData{})
}
