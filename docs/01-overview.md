# 1. Overview & Architecture

A **real-time forum** (single-page web app) written in **Go** with a **SQLite**
database and a **vanilla JavaScript** frontend. Users register, log in, write
posts with categories, comment, like/dislike, and send private messages to each
other in real time over WebSockets.

## Big-picture architecture

```
Browser (web/index.html + 4 JS files)
   │  HTTP JSON calls (/api/...)  +  WebSocket (/ws)
   ▼
Go server (main.go, port 8080)
   ├── internals/auth      → register, login, sessions, middleware
   ├── internals/forum     → posts, comments, categories, reactions
   ├── internals/chat      → private messages + WebSocket hub
   └── internals/database  → opens SQLite, runs schema.sql, seeds categories
   ▼
forum.db (SQLite file)
```

## Project layout

```
forum/
├── main.go                     # entry point: routes + server
├── schema.sql                  # all table definitions
├── forum.db                    # the SQLite database file
├── internals/
│   ├── auth/                   # authentication package
│   │   ├── handlers.go         # /api/register, /api/login, /api/logout, /api/me
│   │   ├── session.go          # session cookies, Protect middleware
│   │   ├── middleware.go       # size limits, MaxBody middleware
│   │   └── response.go         # JSON / Error response helpers
│   ├── forum/                  # forum package
│   │   ├── posts.go            # /api/posts (list, create, delete)
│   │   ├── comments.go         # /api/comments (list, create, delete)
│   │   ├── reactions.go        # /api/reactions (like/dislike toggle)
│   │   ├── categories.go       # /api/categories
│   │   └── models.go           # Post, Comment, Category structs
│   ├── chat/                   # chat package
│   │   ├── hub.go              # /ws WebSocket handler + connection registry
│   │   └── handlers.go         # /api/users, /api/messages
│   └── database/
│       ├── db.go               # InitDB: open SQLite, run schema
│       └── seed.go             # default categories
└── web/                        # frontend (served as static files)
    ├── index.html              # the single page (all views inside)
    ├── style.css
    └── js/
        ├── app.js              # SPA core: views, api() wrapper, session watch
        ├── auth.js             # login/register/logout forms
        ├── posts.js            # feed, post detail, comments, reactions
        └── chat.js             # sidebar, WebSocket client, chat box
```

## The design principle used everywhere

**The server never trusts the client**: every input is re-validated on the
backend, ownership is checked before deletes, and the session is verified on
every request. The client keeps itself honest too, with the automatic logout
on any 401 response.
