package auth

import (
	"strings"
	"unicode"
	"net/mail"
)

func isValidEmail(email string) bool {
	email = strings.TrimSpace(email)

	_, err := mail.ParseAddress(email)
	return err == nil
}

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
