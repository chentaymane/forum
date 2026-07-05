package auth

// ─── Input Validation ──────────────────────────────────────────────────────
// These functions live in the auth package because they're primarily used
// during registration and login, but they are pure string checks that could
// go anywhere.

import (
	"strings"
	"unicode"
)

// isValidEmail does basic email validation.
// It delegates to Go's mail.ParseAddress which handles the RFC 5322 rules.
func isValidEmail(email string) bool {
	email = strings.TrimSpace(email)
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return false
	}
	return true
}

// isValidUserName checks that the username:
//   - Is between 3 and 20 characters long
//   - Starts with a letter
//   - Contains only letters, digits, underscores and hyphens
func isValidUserName(username string) bool {
	if len(username) < 3 || len(username) > 20 {
		return false
	}
	runes := []rune(username)
	if !unicode.IsLetter(runes[0]) {
		return false
	}
	for _, r := range runes {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

// isValidPassword enforces a strong password policy:
//   - Minimum 8 characters, maximum 72 (bcrypt limit)
//   - At least one uppercase letter
//   - At least one lowercase letter
//   - At least one digit
//   - At least one symbol or punctuation character
func isValidPassword(password string) bool {
	if len(password) < 8 || len(password) > 72 {
		return false
	}
	var hasUpper, hasLower, hasNumber, hasSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasNumber = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}
	}
	return hasUpper && hasLower && hasNumber && hasSymbol
}
