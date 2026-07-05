package auth

// ─── Session Management ────────────────────────────────────────────────────
// Sessions let us remember who a user is across HTTP requests without asking
// for the password every time.  When a user logs in we:
//   1. Generate a random UUID as the session token.
//   2. Store it in the `sessions` table with an expiry date.
//   3. Set it as an HttpOnly cookie so JavaScript can't steal it.
//
// On every subsequent request the browser sends the cookie back, and
// GetUserFromRequest looks it up in the database to find the user ID.

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"forum/internals/database"

	"github.com/gofrs/uuid"
)

// SESSION_COOKIE_NAME is the key used for the session cookie.
const SESSION_COOKIE_NAME = "forum_session"

// sessionDuration controls how long a session stays valid (24 hours).
const sessionDuration = 24 * time.Hour

// CreateSession generates a new session for the given user, inserts it into
// the database, and returns the session ID (a UUIDv4 string).
func CreateSession(userID int) (string, error) {
	// Generate a unique session ID
	u1, err := uuid.NewV4()
	if err != nil {
		return "", fmt.Errorf("failed to generate uuid: %w", err)
	}
	sessionID := u1.String()
	expiresAt := time.Now().Add(sessionDuration)

	// Remove old sessions for the same user (single‑session policy)
	_, _ = database.DB.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)

	// Insert the new row
	_, err = database.DB.Exec(
		`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`,
		sessionID, userID, expiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return sessionID, nil
}

// GetUserIDFromSession looks up a session token in the database and returns
// the associated user ID.  Returns an error if the session is missing or
// expired (expired sessions are automatically cleaned up).
func GetUserIDFromSession(sessionID string) (int, error) {
	var userID int
	var expiresAt time.Time

	err := database.DB.QueryRow(
		`SELECT user_id, expires_at FROM sessions WHERE id = ?`,
		sessionID,
	).Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("session not found")
	}
	if err != nil {
		return 0, fmt.Errorf("database error: %w", err)
	}

	if time.Now().After(expiresAt) {
		_ = DeleteSession(sessionID)
		return 0, fmt.Errorf("session expired")
	}

	return userID, nil
}

// DeleteSession removes a session row from the database.
func DeleteSession(sessionID string) error {
	_, err := database.DB.Exec(`DELETE FROM sessions WHERE id = ?`, sessionID)
	return err
}

// SetSessionCookie attaches a HttpOnly session cookie to the HTTP response.
// The cookie is readable only by the server, never by JavaScript (XSS safe).
func SetSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SESSION_COOKIE_NAME,
		Value:    sessionID,
		Expires:  time.Now().Add(sessionDuration),
		HttpOnly: true,
		Path:     "/",
	})
}

// GetUserFromRequest extracts the user ID from the session cookie in the
// HTTP request.  Returns 0 and an error if the cookie is missing or invalid.
func GetUserFromRequest(r *http.Request) (int, error) {
	cookie, err := r.Cookie(SESSION_COOKIE_NAME)
	if err != nil {
		return 0, err
	}
	return GetUserIDFromSession(cookie.Value)
}

// LoggedIn is a convenience helper – returns true when a valid session exists.
func LoggedIn(r *http.Request) bool {
	id, err := GetUserFromRequest(r)
	return err == nil && id > 0
}
