package auth

// ─── Password Hashing ──────────────────────────────────────────────────────
// We never store plain‑text passwords.  bcrypt adds a random salt and runs
// many rounds of key derivation so that even if the database is leaked, the
// original passwords are extremely difficult to recover.

import "golang.org/x/crypto/bcrypt"

// HashPassword returns a bcrypt hash of the plain‑text password.
// The cost factor (14) means ~1‑2 seconds per hash on modern hardware – slow
// enough to frustrate brute‑force attacks, fast enough for a login page.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// CheckPasswordHash compares a plain‑text password against a bcrypt hash.
// Returns true if they match.
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
