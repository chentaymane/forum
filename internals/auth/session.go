package auth

import (
	"net/http"
	"time"

	"rtforum/internals/database"

	"github.com/gofrs/uuid"
)

// SESSION_COOKIE_NAME is the name of the cookie holding the session id.
const SESSION_COOKIE_NAME = "rtf_session"

// UserID returns the logged in user's id from the session cookie (0 if none
// or if the session has expired).
func UserID(r *http.Request) int {
	cookie, err := r.Cookie(SESSION_COOKIE_NAME)
	if err != nil {
		return 0
	}
	var id int
	var expiresAt time.Time
	err = database.DB.QueryRow(`SELECT user_id, expires_at FROM sessions WHERE id = ?`, cookie.Value).Scan(&id, &expiresAt)
	if err != nil || time.Now().After(expiresAt) {
		return 0
	}
	return id
}

// Protect wraps a handler so only logged in users can reach it.
func Protect(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if UserID(r) == 0 {
			Error(w, http.StatusUnauthorized, "not logged in")
			return
		}
		next(w, r)
	}
}

// createSession creates a session for the user and sets the cookie.
// Any previous session for the same user is removed (one session per user).
func createSession(w http.ResponseWriter, userID int) error {
	u, err := uuid.NewV4()
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(24 * time.Hour)

	database.DB.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	_, err = database.DB.Exec(`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`, u.String(), userID, expiresAt)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SESSION_COOKIE_NAME,
		Value:    u.String(),
		Expires:  expiresAt,
		HttpOnly: true,
		Path:     "/",
	})
	return nil
}
