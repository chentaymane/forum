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

	err := http.ListenAndServe(port, mux)
	if err != nil {
		return err
	}
	return nil
}
