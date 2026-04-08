package server

import (
	"context"
	"database/sql"
	_ "embed"
	"log"
	"net/http"

	"forum-backend/internal/db"

	_ "github.com/mattn/go-sqlite3"
)

const port = ":8080"

var dataBase *db.Queries

//go:embed schema.sql
var ddl string

func Init() error {
	dataFile, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		panic(err)
	}
	_, err = dataFile.ExecContext(context.Background(), ddl)
	if err != nil {
		panic(err)
	}
	log.Println(ddl)
	dataBase = db.New(dataFile)
	makeLimiter()
	parseHTML()

	mux := http.NewServeMux()
	mux.Handle("/", Middleware(http.HandlerFunc(HelloWorld)))
	mux.Handle("/register", Middleware(http.HandlerFunc(register)))
	mux.Handle("/login", Middleware(http.HandlerFunc(login)))
	mux.Handle("/style.css", static())
	err = http.ListenAndServe(port, mux)
	if err != nil {
		return err
	}
	return nil
}

func static() http.Handler {
	fs := http.FileServer(http.Dir("web/static/"))
	return fs
}
