package server

import (
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"strings"

	"forum-backend/internal/db"
)

type data struct {
	User  *db.User
	Error error
}

func HelloWorld(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "404", 404)
		return
	}
	var d data
	ctx := r.Context()
	us := ctx.Value("user")
	if us != nil {
		user := us.(*db.GetSessionRow)
		d.User = &db.User{
			Username: strings.Title(user.Username),
		}
	}

	parsedPages.ExecuteTemplate(w, "hello", d)
}

func getValues(values url.Values, reg bool) ([]string, error) {
	username := values.Get("username")
	if (username == "" || len(username) == 1 || strings.Contains(username, " ")) && reg {
		return nil, fmt.Errorf("invalid username")
	}

	email := values.Get("email")
	if email == "" {
		return nil, fmt.Errorf("empty")
	}

	_, err := mail.ParseAddress(email)
	if err != nil {
		return nil, fmt.Errorf("invalid email")
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
		values, err := getValues(r.PostForm, true)
		if err != nil {
			log.Println(err)
			w.Write([]byte("error"))
		}
		err = registerUser(values[0], values[1], values[2])
		if err != nil {
			log.Println(err)
		}
		w.Write([]byte("good"))
	}
}

func login(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		parsedPages.ExecuteTemplate(w, "login", nil)
	case http.MethodPost:
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Bad Request", 400)
		}
		values, err := getValues(r.PostForm, false)
		if err != nil {
			log.Println(err)
			w.Write([]byte("error"))
		}
		id, err := LogInUser(values[1], values[2])
		if err != nil {
			log.Println(err)
			http.Error(w, "401", http.StatusNonAuthoritativeInfo)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "session_id",
			Value:    id,
			Secure:   true,
			HttpOnly: true,
			Path:     "/",
			SameSite: http.SameSiteDefaultMode,
		})
		w.Write([]byte(id))
	}
}
