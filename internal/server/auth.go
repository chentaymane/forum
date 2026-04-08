package server

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"forum-backend/internal/db"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type client struct {
	ip    string
	timer *time.Timer
	reqs  uint8
}
type ratelimiter struct {
	clients map[string]*client
	mu      sync.RWMutex
}

var limiter ratelimiter

func makeLimiter() {
	limiter.clients = make(map[string]*client)
}

func limitCheck(ip string) bool {
	limiter.mu.RLock()
	if c, ok := limiter.clients[ip]; ok {
		if c.reqs >= 60 {
			limiter.mu.RUnlock()
			return false
		}
		c.reqs++
		c.timer.Reset(1 * time.Minute)
		limiter.mu.RUnlock()
		return true
	}
	limiter.mu.RUnlock()
	limiter.mu.Lock()
	t := time.NewTimer(1 * time.Minute)
	c := client{
		ip:    ip,
		reqs:  1,
		timer: t,
	}
	go func() {
		<-c.timer.C

		limiter.mu.Lock()
		delete(limiter.clients, c.ip)
		limiter.mu.Unlock()
	}()
	limiter.clients[c.ip] = &c
	limiter.mu.Unlock()
	return true
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Adding rate  ratelimiter
		if !limitCheck(r.RemoteAddr) {
			http.Error(w, "Many Request", http.StatusTooManyRequests)
			log.Printf("this man stopped: %s\n", r.RemoteAddr)
			return
		}
		c, err := r.Cookie("session_id")
		if err == nil {
			user, err := getUser(c.Value)
			if err == nil {
				ctx := context.WithValue(r.Context(), "user", user)
				g := r.WithContext(ctx)
				r = g
				return
			}
		}
		log.Printf("This man passed: %s\n", r.RemoteAddr)

		next.ServeHTTP(w, r)
	})
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

func LogInUser(email string, password string) (string, error) {
	user, err := dataBase.GetUser(context.Background(), email)
	if err != nil {
		return "", err
	}
	err = bcrypt.CompareHashAndPassword(user.Password, []byte(password))
	if err != nil {
		return "", err
	}
	dataBase.DeleteUserSession(context.Background(), user.ID)
	id, err := dataBase.CreateSession(context.Background(), db.CreateSessionParams{
		ID:     []byte(uuid.New().String()),
		UserID: user.ID,
	})
	if err != nil {
		// log.Println(err)
		return "", err
	}
	// return session_id
	return string(id), err
}

func getUser(id string) (*db.GetSessionRow, error) {
	user, err := dataBase.GetSession(context.Background(), []byte(id))
	if err != nil {
		return nil, err
	}
	return &user, nil
}
