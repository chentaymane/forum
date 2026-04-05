package server

import (
	"net/http"
)

const port = ":8080"

func Init() error {
	router := http.NewServeMux()
	err := http.ListenAndServe(port, router)
	if err != nil {
		return err
	}
	return nil
}
