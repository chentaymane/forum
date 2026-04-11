package middleware

import (
	"forum/internals/auth"
	"forum/internals/errors"
	"net/http"
)

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Check if user exists in session/cookie
		_, err := auth.GetUserFromRequest(r)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// User is logged in → continue
		next(w, r)
	}
}

func Guest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if auth.LoggedIn(r) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		// User is NOT logged in → continue
		next(w, r)
	}
}

func Method(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != method {
			// TODO RENDER ERROR
			errors.RenderError(w, http.StatusText(405), 405)
			return
		}
		next(w, r)
	}
}
