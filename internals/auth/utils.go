package auth

// ─── Input Validation Utilities ────────────────────────────────────────────
//
// These functions validate raw user input. They are used during registration
// and login to ensure data meets minimum requirements BEFORE it reaches the
// database. The spa package has its own parallel validators — these live here
// for use by the auth package itself if needed.
//
// SECURITY:
// All validation happens server-side. The frontend may also validate, but
// we never trust the client — attackers can bypass browser checks.

import (
	"strings"
	"unicode"
)

// isValidEmail does basic email validation.
// Checks that the string contains "@" and "." — a simple but effective
// first-pass filter. Full RFC 5322 validation is overkill for a forum.
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
//   - Contains only letters, digits, underscores, and hyphens
//
// This prevents special characters that could cause issues in URLs or queries.
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
//   - Minimum 8 characters, maximum 72 (bcrypt truncates at 72)
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
