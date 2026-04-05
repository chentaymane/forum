package main

import (
	"forum-backend/internal/server"
)

func main() {
	err := server.Init()
	if err != nil {
		panic(err)
	}
}
