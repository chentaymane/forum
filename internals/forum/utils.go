package forum

import (
	"database/sql"
	"fmt"
	"forum/internals/database"
	"forum/internals/errors"
	"net/http"
	"strconv"
)

// DeleteWithOwnershipCheck is a generic deletion handler that verifies ownership before deleting.
// It handles the common pattern of: check ownership → delete → redirect
//
// Parameters:
// - w: http.ResponseWriter
// - r: *http.Request
// - table: database table name ("posts" or "comments")
// - idParamName: form parameter name ("post_id" or "comment_id")
// - id: the ID to delete
// - userID: the authenticated user's ID
// - redirectURL: where to redirect after deletion (empty string = referer with fallback to "/")
// - itemName: human-readable name for error messages ("post" or "comment")
func DeleteWithOwnershipCheck(w http.ResponseWriter, r *http.Request, table string, idParamName string, id int, userID int, redirectURL string, itemName string) {
	if id <= 0 {
		errors.RenderError(w, fmt.Sprintf("Invalid %s ID", itemName), http.StatusBadRequest)
		return
	}

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
	if redirectURL == "" {
		// Use referer with fallback to "/"
		redirectURL = r.Header.Get("Referer")
		if redirectURL == "" {
			redirectURL = "/"
		}
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}

func parseCatsQuery(categoryIDs []string) ([]int, error) {
	seen := make(map[int]bool)
	var uniqueIDs []int

	for _, idStr := range categoryIDs {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return nil, fmt.Errorf("invalid category id")
		}

		if seen[id] {
			continue
		}
		seen[id] = true

		// Check existence
		var exists int
		err = database.DB.QueryRow(
			"SELECT 1 FROM categories WHERE id = ?",
			id,
		).Scan(&exists)

		if err != nil {
			return nil, fmt.Errorf("category does not exist")
		}

		uniqueIDs = append(uniqueIDs, id)
	}

	if len(uniqueIDs) == 0 {
		return nil, fmt.Errorf("at least one category required")
	}

	return uniqueIDs, nil
}
