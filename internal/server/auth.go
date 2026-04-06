package server

import (
	"log"
	"net/http"
	"sync"
	"time"
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
		log.Printf("This man passed: %s\n", r.RemoteAddr)

		next.ServeHTTP(w, r)
	})
}
