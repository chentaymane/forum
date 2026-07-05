package errors

// ─── Why this package exists ──────────────────────────────────────────────
// In an SPA, most errors are returned as JSON to the JavaScript frontend, so
// the browser can show a toast or inline message.  This package provides the
// small helper that the old server‑side template renderer used – it's kept
// only because the forum package still calls it in a few legacy spots that
// we haven't moved to JSON yet.

import (
	"bytes"
	"html/template"
	"net/http"
)

// RenderError writes an HTML error page directly.  New code should return
// JSON instead and let the SPA handle rendering.
func RenderError(w http.ResponseWriter, message string, code int) {
	tmpl, err := template.New("error").Parse(page())
	if err != nil {
		http.Error(w, message, code)
		return
	}
	var buf bytes.Buffer
	_ = tmpl.Execute(&buf, map[string]any{"Code": code, "Message": message})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write(buf.Bytes())
}

func page() string {
	return `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>Error</title></head>
<body style="font-family:sans-serif;display:grid;place-items:center;min-height:100vh;background:#0f172a;color:#e5eefb;">
<div style="text-align:center"><h1>{{.Code}}</h1><p>{{.Message}}</p><a href="/" style="color:#7dd3fc;">Go home</a></div>
</body></html>`
}
