package auth

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	database "forum/db"

	"github.com/gofrs/uuid"
)

const SESSION_COOKIE_NAME = "forum_session"

// CreateSession creates a new session for a user and returns its ID.
func CreateSession(userID int) (string, error) {
	u1, err := uuid.NewV4()
	if err != nil {
		return "", fmt.Errorf("failed to generate uuid: %w", err)
	}
	sessionID := u1.String()
	expiresAt := time.Now().Add(24 * time.Hour) // Session valid for 24 hours

	query := `INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`
	_, err = database.DB.Exec(query, sessionID, userID, expiresAt)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return sessionID, nil
}

// GetUserIDFromSession retrieves the user ID associated with a session ID.
func GetUserIDFromSession(sessionID string) (int, error) {
	var userID int
	var expiresAt time.Time

	query := `SELECT user_id, expires_at FROM sessions WHERE id = ?`
	err := database.DB.QueryRow(query, sessionID).Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("session not found")
	}
	if err != nil {
		return 0, fmt.Errorf("database error: %w", err)
	}

	if time.Now().After(expiresAt) {
		DeleteSession(sessionID)
		return 0, fmt.Errorf("session expired")
	}

	return userID, nil
}

// DeleteSession removes a session from the database.
func DeleteSession(sessionID string) error {
	query := `DELETE FROM sessions WHERE id = ?`
	_, err := database.DB.Exec(query, sessionID)
	return err
}

// SetSessionCookie sets a session cookie in the response.
func SetSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SESSION_COOKIE_NAME,
		Value:    sessionID,
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Path:     "/",
	})
}

// GetUserFromRequest returns the user ID from the session cookie in the request.
func GetUserFromRequest(r *http.Request) (int, error) {
	cookie, err := r.Cookie(SESSION_COOKIE_NAME)
	if err != nil {
		return 0, err
	}
	return GetUserIDFromSession(cookie.Value)
}
