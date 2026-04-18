package forum

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"forum/internals/database"
	"forum/internals/errors"
)

// - itemName: human-readable name for error messages ("post" or "comment")
func deleteWithOwnershipCheck(w http.ResponseWriter, r *http.Request, table string, idParamName string, id int, userID int, itemName string) {
	if id <= 0 {
		errors.RenderError(w, fmt.Sprintf("Invalid %s ID", itemName), http.StatusBadRequest)
		return
	}
	switch table {
	case "comments", "posts":

		tx, err := database.DB.Begin()
		if err != nil {
			errors.RenderError(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		// Check ownership
		var ownerID int
		query := fmt.Sprintf("SELECT user_id FROM %s WHERE id = ?", table)
		err = tx.QueryRow(query, id).Scan(&ownerID)
		if err == sql.ErrNoRows {
			errors.RenderError(w, fmt.Sprintf("%s not found", capitalizeFirst(itemName)), http.StatusNotFound)
			return
		}
		if err != nil {
			errors.RenderError(w, "Database error", http.StatusInternalServerError)
			return
		}

		if ownerID != userID {
			errors.RenderError(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Delete from table
		deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE id = ?", table)
		result, err := tx.Exec(deleteQuery, id)
		if err != nil {
			errors.RenderError(w, fmt.Sprintf("Failed to delete %s", itemName), http.StatusInternalServerError)
			return
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			errors.RenderError(w, fmt.Sprintf("%s not found", capitalizeFirst(itemName)), http.StatusNotFound)
			return
		}

		if err = tx.Commit(); err != nil {
			errors.RenderError(w, fmt.Sprintf("Failed to commit deletion of %s", itemName), http.StatusInternalServerError)
			return
		}

		// Redirect
		redirectURL := r.Header.Get("Referer")
		if redirectURL == "" || (strings.Contains(redirectURL, "/post") && table == "posts") {
			redirectURL = "/"
		}

		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	default:
		errors.RenderError(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}

// Add this helper function
func FormatDate(raw string) string {
	// Try common SQLite datetime formats
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}
	for _, layout := range formats {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Format("2 Jan 2006 · 15:04")
		}
	}
	return raw // fallback: return as-is if parsing fails
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
