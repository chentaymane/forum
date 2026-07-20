# 8. End-to-End Flows

Two complete flows to tie everything together.

## Logging in

```
Browser                          Go server                        SQLite
───────                          ─────────                        ──────
login form submit
  └─ POST /api/login ──────────► LoginHandler
     {identifier, password}        ├─ SELECT user by nickname/email ──► users
                                   ├─ bcrypt.CompareHashAndPassword
                                   └─ createSession
                                        ├─ DELETE old session ────────► sessions
                                        ├─ INSERT new session ────────► sessions
                                        └─ Set-Cookie forum_session (HttpOnly)
  ◄── {id, nickname} ────────────
JS stores `me`, calls enterForum()
  ├─ loadCategories()  ─► GET /api/categories
  ├─ loadPosts()       ─► GET /api/posts?offset=0
  └─ initChat()        ─► WebSocket /ws  +  GET /api/users
```

Every later request carries the cookie automatically; `Protect` +
`UserID(r)` validate it server-side each time.

## Sending a chat message

```
Sender's tab                     Go server                        Receiver's tabs
────────────                     ─────────                        ───────────────
type + submit
  └─ ws.send({type:"message",
              to, content}) ───► WsHandler read loop
                                   ├─ auth.UserID(r) still valid?
                                   ├─ validate: to>0, not self,
                                   │            non-empty, ≤500 chars
                                   └─ deliver()
                                        ├─ INSERT INTO messages ─► SQLite
                                        ├─ sendTo(receiver) ─────────────► onmessage
                                        └─ sendTo(sender)  ──► onmessage      │
                                                                │             │
                        appears in my chat box (echo) ◄─────────┘             │
                                              open chat? append + scroll ◄────┤
                                              other chat? "new" badge  ◄──────┘
```

## The design principle

The same idea runs through the whole codebase:

- **The server never trusts the client** — every input is re-validated,
  ownership is checked before deletes, the session is verified per request
  (and per WebSocket message).
- **The client keeps itself honest** — any 401 response triggers an
  automatic logout back to the login page.
