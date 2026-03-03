package auth

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"forum/database"
)

// RegisterHandler handles user registration.
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	username := strings.ToLower(strings.TrimSpace(r.FormValue("username")))
	password := r.FormValue("password")

	if email == "" || username == "" || password == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	// Check if email already exists
	var existingEmail string
	err := database.DB.QueryRow("SELECT email FROM users WHERE email = ?", email).Scan(&existingEmail)
	if err == nil {
		http.Error(w, "Email already taken", http.StatusConflict)
		return
	} else if err != sql.ErrNoRows {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Hash password
	hashedPassword, err := HashPassword(password)
	if err != nil {
		http.Error(w, "Encryption error", http.StatusInternalServerError)
		return
	}

	// Insert user
	res, err := database.DB.Exec(
		"INSERT INTO users (email, username, password) VALUES (?, ?, ?)",
		email, username, hashedPassword,
	)
	if err != nil {
		http.Error(w, "Failed to register user", http.StatusInternalServerError)
		return
	}

	// Get inserted user ID
	userID64, err := res.LastInsertId()
	if err != nil {
		http.Error(w, "Failed to get user ID", http.StatusInternalServerError)
		return
	}
	userID := int(userID64)

	//  Create session
	sessionID, err := CreateSession(userID)
	if err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	//  Set cookie
	SetSessionCookie(w, sessionID)
	
	// Redirect to home page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// LoginHandler handles user login.
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")

	var userID int
	var hashedPassword string
	err := database.DB.QueryRow("SELECT id, password FROM users WHERE email = ?", email).Scan(&userID, &hashedPassword)
	if err == sql.ErrNoRows {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if !CheckPasswordHash(password, hashedPassword) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Create session
	sessionID, err := CreateSession(userID)
	if err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Set cookie
	SetSessionCookie(w, sessionID)

	// Redirect to home page after successful login
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// LogoutHandler handles user logout.
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(SESSION_COOKIE_NAME)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	DeleteSession(cookie.Value)

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     SESSION_COOKIE_NAME,
		Value:    "",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Path:     "/",
	})

	// Redirect to home page after logout
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
