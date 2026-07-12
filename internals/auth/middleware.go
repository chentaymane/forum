package auth

import "net/http"

// Size limits for user submitted text, enforced by the handlers.
const (
	MaxNameLen     = 30   // nickname, first and last name
	MaxEmailLen    = 100  //
	MaxPasswordLen = 72   // bcrypt rejects anything longer
	MaxTitleLen    = 100  // post title
	MaxContentLen  = 2000 // post body
	MaxCommentLen  = 1000 // comment body
	MaxMessageLen  = 500  // chat message
)

// maxBodyBytes caps the size of any JSON request body.
const maxBodyBytes = 8 << 10 // 8 KB

// MaxBody is a middleware that stops reading a request body past maxBodyBytes,
// so oversized requests fail to decode instead of being loaded into memory.
func MaxBody(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		next(w, r)
	}
}
