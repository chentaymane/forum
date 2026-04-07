package server

import (
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
)

func HelloWorld(w http.ResponseWriter, r *http.Request) {
	parsedPages.ExecuteTemplate(w, "hello", nil)
}

func getValues(values url.Values) ([]string, error) {
	username := values.Get("username")
	if username == "" {
		return nil, fmt.Errorf("empty")
	}
	email := values.Get("email")
	if email == "" {
		return nil, fmt.Errorf("empty")
	}
	_, err := mail.ParseAddress(email)
	if err != nil {
		return nil, fmt.Errorf("Invalid email")
	}
	password := values.Get("password")
	if password == "" {
		return nil, fmt.Errorf("empty")
	}
	return []string{username, email, password}, nil
}

func register(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		parsedPages.ExecuteTemplate(w, "register", nil)
	case http.MethodPost:
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Bad Request", 400)
		}
		values, err := getValues(r.PostForm)
		if err != nil {
			log.Println(err)
			w.Write([]byte("error"))
		}
		str := strings.Join(values, " ")
		w.Write([]byte(str))
	}
}

func login(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		parsedPages.ExecuteTemplate(w, "login", nil)
	}
}
