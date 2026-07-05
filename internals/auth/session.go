package auth

// ─── Session Management ──────────────────────────────────────────────────────
//
// Sessions let us remember who a user is across HTTP requests without asking
// for the password every time. When a user logs in or registers, we:
//   1. Generate a random UUID as the session token.
//   2. Store it in the `sessions` table with an expiry date (24 hours).
//   3. Set it as an HttpOnly + SameSite=Lax cookie.
//
// On every subsequent request, the browser sends the cookie back. We look up
// the session in the database to find the user ID.
//
// SECURITY:
// - Session IDs are UUIDv4 (random, unpredictable).
// - Cookies are HttpOnly (JavaScript can't read them → XSS-safe).
// - Cookies are SameSite=Lax (prevents CSRF from other origins).
// - Old sessions are deleted when a user logs in again (single-session policy).
// - Expired sessions are automatically cleaned up on lookup.

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"forum/internals/database"

	"github.com/gofrs/uuid"
)

// SESSION_COOKIE_NAME is the key used for the session cookie.
// The browser stores this as "forum_session=<uuid>".
const SESSION_COOKIE_NAME = "forum_session"

// sessionDuration controls how long a session stays valid (24 hours).
// After this time, the user must log in again.
const sessionDuration = 24 * time.Hour

// CreateSession generates a new session for the given user, inserts it into
// the database, and returns the session ID (a UUIDv4 string).
//
// SECURITY: We delete ALL existing sessions for this user first (single-
// session policy). This means logging in on a new device invalidates any
// previous sessions.
func CreateSession(userID int) (string, error) {
	// Generate a unique, unpredictable session ID
	u1, err := uuid.NewV4()
	if err != nil {
		return "", fmt.Errorf("failed to generate uuid: %w", err)
	}
	sessionID := u1.String()
	expiresAt := time.Now().Add(sessionDuration)

	// Remove old sessions for the same user (single-session policy)
	// This prevents old stolen session tokens from working after a re-login
	_, _ = database.DB.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)

	// Insert the new session row
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
// the associated user ID. Returns an error if the session is missing or
// expired (expired sessions are automatically cleaned up).
//
// SECURITY: This is called on every authenticated request. Expired sessions
// are deleted immediately when detected, preventing reuse.
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

	// Check if session has expired
	if time.Now().After(expiresAt) {
		_ = DeleteSession(sessionID) // clean up expired session
		return 0, fmt.Errorf("session expired")
	}

	return userID, nil
}

// DeleteSession removes a session row from the database.
// Called during logout and when expired sessions are detected.
func DeleteSession(sessionID string) error {
	_, err := database.DB.Exec(`DELETE FROM sessions WHERE id = ?`, sessionID)
	return err
}

// SetSessionCookie attaches a secure session cookie to the HTTP response.
//
// SECURITY:
// - HttpOnly: JavaScript cannot access this cookie (prevents XSS theft).
// - SameSite=Lax: Cookie is only sent for same-origin requests (prevents CSRF).
// - Path="/": Cookie is sent for all paths on this domain.
// - No Domain set: Cookie is only sent to the origin that set it.
func SetSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SESSION_COOKIE_NAME,
		Value:    sessionID,
		Expires:  time.Now().Add(sessionDuration),
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})
}

// GetUserFromRequest extracts the user ID from the session cookie in the
// HTTP request. Returns 0 and an error if the cookie is missing or invalid.
//
// This is the main entry point for session lookup — called by every handler
// that needs to identify the current user.
func GetUserFromRequest(r *http.Request) (int, error) {
	cookie, err := r.Cookie(SESSION_COOKIE_NAME)
	if err != nil {
		return 0, err
	}
	return GetUserIDFromSession(cookie.Value)
}

// LoggedIn is a convenience helper — returns true when a valid session exists
// for the current request. Used for optional authentication (e.g., showing
// different content to logged-in vs anonymous users).
func LoggedIn(r *http.Request) bool {
	id, err := GetUserFromRequest(r)
	return err == nil && id > 0
}
