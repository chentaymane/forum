# Real Time Forum

Single page forum with registration/login, posts, comments and real time
private messages (websockets).

## Stack
- **Golang**: HTTP API + Gorilla websockets
- **SQLite**: data storage (`forum.db`, created from `schema.sql`)
- **Javascript / HTML / CSS**: single page application (one HTML file)

## Run
```
go run .
```
Then open http://localhost:8080

Note: `github.com/mattn/go-sqlite3` needs a C compiler (CGO). On Windows
without gcc, either use Docker (`docker build -t rtf . && docker run -p 8080:8080 rtf`)
or install gcc (e.g. w64devkit / MSYS2).

## Structure
- `main.go` — routes
- `internals/database` — SQLite init
- `internals/auth` — register, login (nickname or e-mail), logout, sessions (bcrypt + uuid)
- `internals/forum` — posts, categories, comments
- `internals/chat` — websocket hub, users list, private messages (10 by 10)
- `web/` — index.html, style.css, js/ (SPA, no frameworks)
