package server

import (
	"net/http"
)

const port = ":8080"

func Init() error {
	makeLimiter()
	parseHTML()

	mux := http.NewServeMux()
	mux.Handle("/", Middleware(http.HandlerFunc(HelloWorld)))
	mux.Handle("/register", Middleware(http.HandlerFunc(register)))
	mux.Handle("/login", Middleware(http.HandlerFunc(login)))
	mux.Handle("/style.css", static())
	err := http.ListenAndServe(port, mux)
	if err != nil {
		return err
	}
	return nil
}

func static() http.Handler {
	fs := http.FileServer(http.Dir("web/static/"))
	return fs
}
