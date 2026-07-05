package middleware

// ─── HTTP Middleware ────────────────────────────────────────────────────────
// Middleware functions wrap http.HandlerFunc to add cross-cutting behaviour.
//
// The SPA currently only needs Method() – it enforces the HTTP method before
// the handler is called. Without it, a POST handler would silently accept
// GET requests and return confusing responses.
//
// AuthMiddleware and Guest are kept from the old multi-page version in case
// you want to add protected routes later.

import "net/http"

// Method returns a handler that rejects requests whose HTTP method does not
// match the expected `method`. Returns 405 Method Not Allowed on mismatch.
//
// Usage:
//
//	http.HandleFunc("/api/posts", middleware.Method("GET", app.PostsHandler))
func Method(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		next(w, r)
	}
}
