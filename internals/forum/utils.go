package forum

// ─── Shared Utilities ──────────────────────────────────────────────────────
// Small helpers used across the forum package.

import "time"

// FormatDate tries to parse a date string coming from SQLite and return a
// human‑friendly short format like "2 Jan 2006 · 15:04".  If parsing fails
// it returns the raw string unchanged.
func FormatDate(raw string) string {
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
	return raw
}
