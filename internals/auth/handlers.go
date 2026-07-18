package auth

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"rtforum/internals/database"

	"golang.org/x/crypto/bcrypt"
)

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

	// Validate all fields (presence and size) before touching the database.
	if _, err := mail.ParseAddress(in.Email); err != nil || len(in.Email) > MaxEmailLen ||
		in.Nickname == "" || len(in.Nickname) > MaxNameLen ||
		in.FirstName == "" || len(in.FirstName) > MaxNameLen ||
		in.LastName == "" || len(in.LastName) > MaxNameLen ||
		in.Gender == "" || in.Age < 1 || in.Age > 120 ||
		len(in.Password) < 6 || len(in.Password) > MaxPasswordLen {
		Error(w, http.StatusBadRequest, "invalid input")
		return
	}

	// Never store the plain password, only its bcrypt hash.
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
	// Same "invalid credentials" reply whether the user is missing or the
	// password is wrong, so we don't leak which nicknames exist.
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
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	http.SetCookie(w, &http.Cookie{
		Name:     CHECK_COOKIE_NAME,
		Value:    "",
		Expires:  time.Unix(0, 0),
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// MeHandler returns the logged in user (used on page load to restore the session).
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
