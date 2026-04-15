# Forum

A lightweight, full-stack web forum built entirely in Go with an SQLite database. No framework, no ORM, no JavaScript runtime — just a single compiled binary that serves HTML templates, handles sessions, and manages a relational database.

---

## Table of Contents

- [Features](#features)
- [Tech Stack](#tech-stack)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)
- [Docker](#docker)
- [Configuration](#configuration)
- [Database](#database)
- [Authentication & Sessions](#authentication--sessions)
- [Validation Rules](#validation-rules)
- [API Routes](#api-routes)
- [Middleware](#middleware)
- [Pagination & Filtering](#pagination--filtering)
- [Reactions System](#reactions-system)
- [Template System](#template-system)
- [Error Handling](#error-handling)

---

## Features

- **Register & Login** — Email/password auth with strict validation rules
- **Session management** — UUID-based sessions stored in SQLite with 24-hour expiry
- **Create posts** — Title, content, and one or more categories required
- **Delete posts** — Only the post owner can delete their post
- **Comments** — Add and delete comments on any post (owner-only delete)
- **Reactions** — Like (`+1`) or dislike (`-1`) posts and comments
  - Clicking the same reaction again **removes** it (toggle)
  - Clicking the opposite reaction **switches** it
- **Category filtering** — Filter the home feed by category
- **Personal filters** — View your own posts, posts you reacted to, or posts you commented on
- **Pagination** — Home feed is paginated (10 posts per page)
- **Guest access** — Guests can browse posts and comments but cannot interact
- **Friendly error pages** — Custom HTML error pages for all error codes

---

## Tech Stack

| Layer       | Technology                               |
|-------------|------------------------------------------|
| Language    | Go 1.24                                  |
| Database    | SQLite 3 via `mattn/go-sqlite3` (CGO)    |
| Sessions    | UUID v4 via `gofrs/uuid`                 |
| Passwords   | bcrypt cost 14 via `golang.org/x/crypto` |
| Templates   | Go standard `html/template`              |
| Styles      | Vanilla CSS (single file)                |
| Routing     | Go standard `net/http` ServeMux          |
| Container   | Docker multi-stage build                 |

---

## Project Structure

```
.
├── main.go                          # Entry point and route registration
├── schema.sql                       # Full DB schema, indexes, and triggers
├── go.mod
├── go.sum
├── Dockerfile
│
├── internals/
│   ├── auth/
│   │   ├── handlers.go              # Register, Login, Logout HTTP handlers
│   │   ├── session.go               # Session create / get / delete / cookie
│   │   ├── encryption.go            # bcrypt hash and compare
│   │   └── utils.go                 # Email, username, password validators
│   │
│   ├── database/
│   │   └── db.go                    # SQLite init, schema load, category seed
│   │
│   ├── errors/
│   │   └── errors.go                # RenderError — renders error.html with code + message
│   │
│   ├── forum/
│   │   ├── posts.go                 # Post struct, GetPosts, GetPostsCount, GetPostCategories
│   │   ├── comments.go              # Comment struct, CreateComment, DeleteComment, GetCommentsByPost
│   │   └── likes.go                 # ReactionsHandler, GetLikesCount, insertReaction
│   │
│   └── handlers/
│       ├── home.go                  # HomeHandler — feed with filters and pagination
│       ├── post.go                  # PostDetails, CreatePostPageHandler
│       ├── models.go                # PageData, PostDetailData, User, Category structs
│       └── utils.go                 # renderTemplate, getTemplate (cached), getCategories
│
└── web/
    ├── static/
    │   └── style.css                # All styles in one file
    └── templates/
        ├── layout.html              # Base HTML layout (nav, footer)
        ├── index.html               # Home feed template
        ├── post_details.html        # Single post view with all comments
        ├── create_post.html         # New post form
        ├── login.html               # Login form
        ├── register.html            # Register form
        └── error.html               # Error page template
```

---

## Getting Started

### Prerequisites

- [Go 1.24+](https://go.dev/dl/)
- GCC — required because `go-sqlite3` uses CGO

On Ubuntu/Debian:
```bash
sudo apt install gcc
```

On macOS:
```bash
xcode-select --install
```

### Run Locally

```bash
git clone https://github.com/your-username/forum.git
cd forum
go mod download
go run .
```

Server starts at **http://localhost:8080**

The database file `forum.db` and schema are created automatically on first run.

---

## Docker

### Build the image

```bash
docker build -t forum .
```

### Run the container

```bash
docker run -p 8080:8080 forum
```

Available at **http://localhost:8080**

### Persist the database across restarts

By default the database lives inside the container and is lost on removal. To keep it on the host:

```bash
docker run -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -e DB_PATH=/app/data/forum.db \
  forum
```

### How the Docker build works

The Dockerfile uses a **two-stage build**:

1. **Builder stage** (`golang:1.24-alpine`) — installs GCC, downloads dependencies, compiles the binary
2. **Runtime stage** (`alpine:latest`) — copies only the binary, templates, static files, and schema

The final image contains no Go toolchain, keeping it minimal.

---

## Configuration

The app reads two optional environment variables:

| Variable      | Default        | Description                       |
|---------------|----------------|-----------------------------------|
| `DB_PATH`     | `./forum.db`   | Path to the SQLite database file  |
| `SCHEMA_PATH` | `./schema.sql` | Path to the SQL schema file       |

---

## Database

The schema is loaded from `schema.sql` on every startup using `IF NOT EXISTS`, so it is safe to run multiple times without data loss.

### Tables

| Table             | Description                                                            |
|-------------------|------------------------------------------------------------------------|
| `users`           | Registered users: email (unique), username (unique), bcrypt password   |
| `posts`           | Forum posts: title (max 60 chars), content, linked to a user           |
| `comments`        | Comments on posts, linked to a user                                    |
| `categories`      | Post categories — seeded with General, Technology, Art, Science        |
| `post_categories` | Many-to-many join between posts and categories                         |
| `reactions`       | Likes/dislikes (`type = 1 or -1`) targeting either a post or a comment |
| `sessions`        | Active sessions: UUID id, user_id, expires_at                          |

### Foreign Keys & Cascades

All child records cascade on delete:
- Deleting a **user** removes their posts, comments, reactions, and sessions
- Deleting a **post** removes its comments, reactions, and category links
- Deleting a **comment** removes its reactions

Foreign key enforcement is explicitly enabled on every startup:
```sql
PRAGMA foreign_keys = ON;
```

### Indexes

Performance indexes are defined for:
- Posts by user and creation date
- Comments by post and by user
- Post-category lookups (both directions)
- Reactions by post and by comment
- Sessions by user and by expiry

### Reaction Triggers

The toggle behavior for reactions is handled **entirely in SQLite** via two `BEFORE INSERT` triggers, with no application-side toggle logic needed:

- `toggle_post_reaction` — fires before inserting a reaction targeting a post
- `toggle_comment_reaction` — fires before inserting a reaction targeting a comment

Logic inside each trigger:
1. If the **same** reaction type already exists → delete it and raise `IGNORE` (cancels the insert)
2. If the **opposite** reaction type exists → delete it, then allow the new insert to proceed

### Seeded Categories

On startup, these four categories are inserted if they do not already exist:

```
General · Technology · Art · Science
```

---

## Authentication & Sessions

### Registration flow

1. Validate email, username, and password (see [Validation Rules](#validation-rules))
2. Check that the email is not already taken
3. Hash the password with **bcrypt at cost 14**
4. Insert the user into `users`
5. Create a UUID v4 session that expires in **24 hours**
6. Set an `HttpOnly` cookie named `forum_session`
7. Redirect to `/`

### Login flow

1. Look up the user by email
2. Compare the submitted password against the stored bcrypt hash using `bcrypt.CompareHashAndPassword`
3. Create a session, set the cookie
4. Redirect to `/`

### Logout flow

1. Read the `forum_session` cookie
2. Delete the session row from the database
3. Overwrite the cookie with an expired value to clear it from the browser
4. Redirect to `/`

### Session lookup (every protected request)

`auth.GetUserFromRequest(r)` is called on every authenticated route:
1. Reads the `forum_session` cookie
2. Queries `sessions` by session ID
3. Checks `expires_at` — if expired, deletes the session and returns an error
4. Returns the `user_id`

---

## Validation Rules

### Email
- Must be a valid RFC 5322 address, validated with Go's `net/mail.ParseAddress`

### Username
- 3–20 characters
- Must start with a letter
- May contain letters, digits, `_`, and `-`

### Password
- 8–72 characters
- Must contain at least one of each: uppercase letter, lowercase letter, digit, symbol

### Post
- Title: 1–60 characters (whitespace trimmed)
- Content: 1–1000 characters (whitespace trimmed)
- At least one category must be selected
- Each submitted category ID must exist in the `categories` table

### Comment
- `post_id` and `content` are both required and must be non-empty

---

## API Routes

### Public — no authentication required

| Method | Path              | Handler               | Description                     |
|--------|-------------------|-----------------------|---------------------------------|
| GET    | `/`               | `HomeHandler`         | Home feed, page 1               |
| GET    | `/page/{page}`    | `HomeHandler`         | Home feed, paginated            |
| GET    | `/post/{post_id}` | `PostDetails`         | Full post with all comments     |
| GET    | `/login`          | `LoginPageHandler`    | Login page                      |
| GET    | `/register`       | `RegisterPageHandler` | Register page                   |

### Auth endpoints — guest only (redirects logged-in users to `/`)

| Method | Path             | Handler           | Description          |
|--------|------------------|-------------------|----------------------|
| POST   | `/auth/register` | `RegisterHandler` | Create a new account |
| POST   | `/auth/login`    | `LoginHandler`    | Log in               |

### Protected — requires a valid session

| Method | Path                   | Handler                     | Description                        |
|--------|------------------------|-----------------------------|------------------------------------|
| GET    | `/create-post`         | `CreatePostPageHandler`     | New post form                      |
| POST   | `/api/posts/create`    | `forum.CreatePost`          | Submit a new post                  |
| POST   | `/api/posts/delete`    | `forum.DeletePost`          | Delete a post (owner only)         |
| POST   | `/api/comments`        | `forum.CreateCommentHandler`| Add a comment                      |
| POST   | `/api/comments/delete` | `forum.DeleteCommentHandler`| Delete a comment (owner only)      |
| POST   | `/api/likes`           | `forum.ReactionsHandler`    | Like or dislike a post or comment  |

All routes enforce their declared HTTP method. Any other method returns **405 Method Not Allowed**.

---

## Middleware

Three middleware wrappers are composed in `main.go`:

### `Method(method, next)`
Enforces a single allowed HTTP method. Returns 405 if the request method does not match.

### `AuthMiddleware(next)`
Checks that the request carries a valid, non-expired session. Returns 401 if unauthenticated.

### `Guest(next)`
Redirects authenticated users to `/`. Applied to login and register pages so logged-in users cannot visit them.

Composition example from `main.go`:
```go
// Guest-only GET page
http.HandleFunc("/login",
    middleware.Method("GET", middleware.Guest(handlers.LoginPageHandler)))

// Protected POST action
http.HandleFunc("/api/posts/create",
    middleware.Method("POST", middleware.AuthMiddleware(forum.CreatePost)))
```

---

## Pagination & Filtering

The home feed supports the following URL query parameters:

| Parameter        | Value | Description                                          |
|------------------|-------|------------------------------------------------------|
| `category_id`    | int   | Filter posts belonging to a specific category        |
| `my_posts`       | `1`   | Show only the logged-in user's posts                 |
| `my_liked_posts` | `1`   | Show only posts the logged-in user has reacted to    |
| `my_comments`    | `1`   | Show only posts the logged-in user has commented on  |

Filters can be combined with category filtering. Pagination is done via path:

```
/           → page 1
/page/2     → page 2
/page/3     → page 3
```

- Page size is **10 posts per page**
- Requesting a page number beyond the last page returns a 404
- The `GetPosts` function builds its SQL query dynamically with `strings.Builder`, adding `JOIN` and `WHERE` clauses only when a filter is active — no unnecessary joins on unfiltered requests

---

## Reactions System

Reactions target either a **post** or a **comment** — never both. The `reactions` table enforces this with a database-level `CHECK` constraint:

```sql
CHECK (
    (post_id IS NOT NULL AND comment_id IS NULL) OR
    (post_id IS NULL AND comment_id IS NOT NULL)
)
```

Toggle behavior summary:

| Situation                          | Result                                  |
|------------------------------------|-----------------------------------------|
| React to something with no reaction | Inserts the reaction                   |
| React with the **same** type again  | Removes the reaction (toggle off)      |
| React with the **opposite** type    | Removes old reaction, inserts new one  |

This is handled entirely by SQLite triggers — the Go handler simply calls `INSERT INTO reactions` and the database does the rest.

After any reaction, the handler redirects back to the `Referer` header URL, or `/` if the header is absent.

---

## Template System

Templates use Go's `html/template` with a **layout + page block** pattern:

- `layout.html` defines a `base` template containing the full HTML shell (header, nav, `<main>`, footer)
- Each page file defines `{{define "title"}}` and `{{define "content"}}` blocks injected into the layout

Templates are **cached in memory** after their first parse, protected by a `sync.RWMutex`. Subsequent requests reuse the cached `*template.Template` without re-reading from disk.

Rendering writes into a `bytes.Buffer` first. If template execution fails after headers have been set, a clean 500 error is returned rather than a partial response reaching the client.

---

## Error Handling

All errors go through a single function:

```go
errors.RenderError(w, message string, code int)
```

It:
1. Parses `web/templates/error.html`
2. Executes it with `ErrorData{Code, Message}`
3. Renders into a `bytes.Buffer`
4. Sets `Content-Type: text/html`, writes the HTTP status code, and flushes the buffer

If the error template itself fails to load (e.g. missing file), it falls back to `http.Error` with plain text.

The error page shows the numeric status code, a human-readable message, and a "Go back" link (`onclick="history.back(); return false;"`) that returns the user to the previous page without requiring any separate script tag.