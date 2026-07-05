package auth

// ─── Password Hashing ──────────────────────────────────────────────────────
//
// We NEVER store plain-text passwords. When a user registers, we hash their
// password with bcrypt and store only the hash. When they log in, we compare
// the provided password against the stored hash.
//
// WHY BCRYPT?
// 1. It automatically generates a random salt for each password (no two
//    hashes of the same password look the same).
// 2. It's intentionally SLOW (cost factor 14 ≈ 1-2 seconds per hash).
//    This makes brute-force attacks impractical even if the database leaks.
// 3. It's been extensively reviewed and is the industry standard for
//    password storage.
//
// SECURITY:
// - The cost factor (14) balances security with UX speed.
// - bcrypt also truncates at 72 bytes, so we limit passwords to 72 chars.
// - The CompareHashAndPassword function is timing-attack resistant.

import "golang.org/x/crypto/bcrypt"

// HashPassword returns a bcrypt hash of the plain-text password.
// The cost factor (14) means ~1-2 seconds per hash on modern hardware — slow
// enough to frustrate brute-force attacks, fast enough for a login page.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// CheckPasswordHash compares a plain-text password against a bcrypt hash.
// Returns true if they match.
//
// bcrypt.CompareHashAndPassword is designed to be constant-time — it doesn't
// leak information about how many characters matched. This prevents timing
// attacks where an attacker could guess the password byte by byte.
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
