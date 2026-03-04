package middleware

import (
	"net/http"
	"forum/auth"
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
