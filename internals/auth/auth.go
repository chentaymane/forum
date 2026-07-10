package auth

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"rtforum/internals/database"

	"github.com/gofrs/uuid"
	"golang.org/x/crypto/bcrypt"
)

const SESSION_COOKIE_NAME = "rtf_session"

// JSON writes a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Error writes a JSON error message.
func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}

// UserID returns the logged in user's id from the session cookie (0 if none).
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

// Protect only lets logged in users through.
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
func createSession(w http.ResponseWriter, userID int) error {
	u, err := uuid.NewV4()
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(24 * time.Hour)

	// One session per user
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

// RegisterHandler creates a new user account and logs it in.
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Nickname  string `json:"nickname"`
		Age       int    `json:"age"`
		Gender    string `json:"gender"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Email     string `json:"email"`
		Password  string `json:"password"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		Error(w, http.StatusBadRequest, "invalid request")
		return
	}

	in.Nickname = strings.TrimSpace(in.Nickname)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.FirstName = strings.TrimSpace(in.FirstName)
	in.LastName = strings.TrimSpace(in.LastName)

	if _, err := mail.ParseAddress(in.Email); err != nil || in.Nickname == "" ||
		in.FirstName == "" || in.LastName == "" || in.Gender == "" ||
		in.Age < 1 || in.Age > 120 || len(in.Password) < 6 {
		Error(w, http.StatusBadRequest, "invalid input")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		Error(w, http.StatusInternalServerError, "encryption error")
		return
	}

	res, err := database.DB.Exec(
		`INSERT INTO users (nickname, age, gender, first_name, last_name, email, password) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		in.Nickname, in.Age, in.Gender, in.FirstName, in.LastName, in.Email, string(hash),
	)
	if err != nil {
		Error(w, http.StatusConflict, "nickname or email already taken")
		return
	}

	id, _ := res.LastInsertId()
	if err := createSession(w, int(id)); err != nil {
		Error(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	JSON(w, http.StatusOK, map[string]any{"id": int(id), "nickname": in.Nickname})
}

// LoginHandler logs a user in with nickname or email + password.
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	in.Identifier = strings.TrimSpace(in.Identifier)

	var id int
	var nickname, hash string
	err := database.DB.QueryRow(
		`SELECT id, nickname, password FROM users WHERE nickname = ? OR email = ?`,
		in.Identifier, strings.ToLower(in.Identifier),
	).Scan(&id, &nickname, &hash)
	if err == sql.ErrNoRows || (err == nil && bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil) {
		Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	} else if err != nil {
		Error(w, http.StatusInternalServerError, "database error")
		return
	}

	if err := createSession(w, id); err != nil {
		Error(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	JSON(w, http.StatusOK, map[string]any{"id": id, "nickname": nickname})
}

// LogoutHandler deletes the session and clears the cookie.
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(SESSION_COOKIE_NAME); err == nil {
		database.DB.Exec(`DELETE FROM sessions WHERE id = ?`, cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SESSION_COOKIE_NAME,
		Value:    "",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Path:     "/",
	})
	JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// MeHandler returns the logged in user (used on page load).
func MeHandler(w http.ResponseWriter, r *http.Request) {
	id := UserID(r)
	if id == 0 {
		Error(w, http.StatusUnauthorized, "not logged in")
		return
	}
	var nickname string
	database.DB.QueryRow(`SELECT nickname FROM users WHERE id = ?`, id).Scan(&nickname)
	JSON(w, http.StatusOK, map[string]any{"id": id, "nickname": nickname})
}
