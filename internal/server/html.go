package server

import (
	"html/template"
)

var parsedPages *template.Template

func parseHTML() {
	parsedPages = template.Must(template.ParseGlob("web/*html"))
}
