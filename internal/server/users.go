package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"strings"

	"forum-backend/internal/db"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type data struct {
	User  *db.User
	Error error
}

func registerUser(user string, email string, password string) error {
	id := uuid.New()
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	err := dataBase.CreateUser(context.Background(), db.CreateUserParams{
		ID:       []byte(id.String()),
		Username: user,

		Email:    email,
		Password: hash,
	})
	return err
}

func HelloWorld(w http.ResponseWriter, r *http.Request) {
	parsedPages.ExecuteTemplate(w, "hello", nil)
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
		str := strings.Join(values, " ")
		w.Write([]byte(str))
	}
}
