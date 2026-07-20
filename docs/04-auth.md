# 4. Auth Package — `internals/auth`

The heart of authentication. The app uses **cookie sessions** stored in the
database, not JWT.

## Sessions — `session.go`

### `newSessionID()`

32 random bytes from `crypto/rand`, hex-encoded → a 64-character unguessable
token (256 bits of entropy).

### `createSession(w, userID)`

1. Deletes any old session for this user — **one session per user**: logging
   in on a second browser kicks out the first.
2. Inserts a new session row valid for **24 hours**.
3. Sets the `forum_session` cookie holding the session id. It is **HttpOnly**
   (JavaScript can't read it, protecting it from XSS theft), uses
   `SameSite=Lax` (blocks most cross-site request forgery) and `Path=/`.

### `UserID(r)`

The function every protected handler calls. Reads the `forum_session` cookie,
looks the session up in the DB, and returns the user id — or `0` if the cookie
is missing, unknown, or expired. **Expired rows are deleted on the spot** so
the sessions table doesn't grow forever.

### `Protect(next)`

Middleware: if `UserID(r) == 0`, respond `401 not logged in` and never call
the real handler.

## Handlers — `handlers.go`

### `RegisterHandler` — `POST /api/register`

1. Decodes the JSON body, trims whitespace, lower-cases the email.
2. Validates **everything before touching the database**: valid email format
   via `mail.ParseAddress`, name lengths ≤ 30, age 1–120, password 6–72 chars.
3. Hashes the password with **bcrypt** — the plaintext password is never
   stored.
4. Inserts the user. If the INSERT fails it's the UNIQUE constraint, so it
   returns `409 nickname or email already taken`.
5. Immediately calls `createSession` — **register = auto-login**.

### `LoginHandler` — `POST /api/login`

- Accepts nickname *or* email in one `identifier` field:
  `WHERE nickname = ? OR email = ?`.
- Verifies with `bcrypt.CompareHashAndPassword`.
- **Security detail:** a missing user and a wrong password return the *same*
  `invalid credentials` message, so attackers can't discover which nicknames
  exist (no user enumeration).

### `LogoutHandler` — `POST /api/logout`

Deletes the session row and expires both cookies (sets them to empty with an
expiry date in the past).

### `MeHandler` — `GET /api/me`

Returns `{id, nickname}` of the current user. The frontend calls it on page
load to restore the session ("am I still logged in?").

## Helpers

### `response.go`

- `JSON(w, status, data)` — writes any value as a JSON response.
- `Error(w, status, msg)` — writes `{"error": "..."}`.

Tiny helpers so every response in the app has the same shape.

### `middleware.go`

The size-limit constants enforced by the handlers:

| Constant | Value | Applies to |
|---|---|---|
| `MaxNameLen` | 30 | nickname, first and last name |
| `MaxEmailLen` | 100 | email |
| `MaxPasswordLen` | 72 | bcrypt rejects anything longer |
| `MaxTitleLen` | 100 | post title |
| `MaxContentLen` | 2000 | post body |
| `MaxCommentLen` | 1000 | comment body |
| `MaxMessageLen` | 500 | chat message |

And **`MaxBody`** — wraps the request body in `http.MaxBytesReader` with an
8 KB cap, so an 8 MB request fails to decode instead of being loaded into RAM.
