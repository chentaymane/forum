package middleware

// ─── HTTP Middleware ────────────────────────────────────────────────────────
// Middleware functions wrap http.HandlerFunc to add cross‑cutting behaviour.
// The SPA currently only needs Method() – AuthMiddleware and Guest are kept
// from the old multi‑page version in case you want to add protected routes.

import "net/http"

// Method returns a handler that rejects requests whose HTTP method does not
// match the expected `method`.
func Method(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		next(w, r)
	}
}
