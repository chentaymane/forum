# Project Documentation

A full explanation of this project, from the entry point to the last function.
Each file covers one part of the codebase:

| File | Covers |
|---|---|
| [01-overview.md](01-overview.md) | What the project is, big-picture architecture |
| [02-entry-point.md](02-entry-point.md) | `main.go` — routes, middleware, server startup |
| [03-database.md](03-database.md) | `internals/database` + `schema.sql` — tables, pragmas, seeding |
| [04-auth.md](04-auth.md) | `internals/auth` — register, login, sessions, cookies, middleware |
| [05-forum.md](05-forum.md) | `internals/forum` — posts, comments, categories, reactions |
| [06-chat.md](06-chat.md) | `internals/chat` — WebSocket hub, private messages |
| [07-frontend.md](07-frontend.md) | `web/` — index.html, app.js, auth.js, posts.js, chat.js |
| [08-flows.md](08-flows.md) | Two end-to-end flows tying everything together |

## What this project is

A **real-time forum** (single-page web app) written in **Go** with a **SQLite**
database and a **vanilla JavaScript** frontend. Users register, log in, write
posts with categories, comment, like/dislike, and send private messages to each
other in real time over WebSockets.
