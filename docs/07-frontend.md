# 7. Frontend — `web/`

## `index.html`

Contains **every view at once**: auth view, error view, feed view, post-detail
view, users sidebar, floating chat box, and the confirm modal. JavaScript just
toggles the `hidden` CSS class — that's the whole "SPA router".

## `app.js` — core / glue

### Startup (`DOMContentLoaded`)

- If the URL isn't `/`, show the 404 view — this is how the server's "always
  serve index.html" trick becomes a real error page.
- Otherwise call `/api/me`: success → `enterForum()`, failure → show the
  login form.

### `api(path, opts)`

The single fetch wrapper everything uses:

- Throws an `Error` on non-OK responses (with the server's `error` message).
- If it ever sees a **401** while logged in, it triggers `logoutLocal()` — so
  an expired session anywhere logs you out cleanly.

### `enterForum()`

Runs after login/register/session-restore: shows the feed, loads categories +
posts, and connects the chat.

### Small utilities

| Function | Purpose |
|---|---|
| `showView(name)` | hide every `.view`, show the requested one |
| `showError(code)` | fill and show the error view (404/405/500...) |
| `post(data)` | build fetch options for a JSON POST |
| `confirmAction(msg)` | promise-based confirm modal, resolves true/false |
| `esc(s)` | HTML-escapes user text — **the XSS protection**, used everywhere content is rendered |
| `throttle(fn, ms)` | rate-limits the scroll handlers |
| `logoutLocal()` | closes the chat, resets state, reloads |

## `auth.js` — login / register / logout

Wires the three forms:

- **Login**: prevent default submit → `POST /api/login` with
  `{identifier, password}` → store the returned user in `me` →
  `enterForum()`. Errors go to `showAuthError`.
- **Register**: same flow with all the profile fields (register auto-logs-in).
- **Logout button**: calls the API, broadcasts `"logout"` to other tabs, and
  resets locally.
- The `show-register` / `show-login` links just swap which form is visible.

## `posts.js` — feed, comments, reactions

### Categories and filters

- **`loadCategories()`** — fills the checkbox list in the compose form and the
  filter pills; also stores `validCatIds` so only real category ids are sent.
- **Filter bar** — one delegated click listener; clicking a pill sets `filter`
  (e.g. `"mine=1"` or `"category=2"`) and reloads the feed.

### Infinite scroll

- **`loadPosts()`** resets the feed and loads the first page.
- **`loadMorePosts()`** fetches `/api/posts?offset=<already loaded>`, appends
  cards, and sets `postsDone` when a page comes back with fewer than 10.
  `loadingPosts` prevents duplicate in-flight requests.
- A throttled window `scroll` listener calls `loadMorePosts()` when you're
  within 300px of the bottom.

### Rendering

- **`renderPostCard(p)`** — one feed card: author, date, category pills,
  title, excerpt, reaction buttons, comment count, and a Delete button **only
  if `me.id === p.userId`**. Clicking the card opens the post.
- **`openPost(id)`** → `renderPostDetail()` + `loadComments()` — the detail
  view with the comment list and comment form.
- All user text goes through `esc()` before being inserted as HTML.

### Reactions

**`react(event, target, id, type)`** — sends the like/dislike, then updates
the two buttons *in place* using the fresh counts the server returns — no page
reload. `event.stopPropagation()` prevents the click from also opening the
post.

### Deleting

**`deletePost` / `deleteComment`** — confirm modal → API call with
`{action: "delete", id}` → remove the element from the DOM and the local
`posts` cache.

## `chat.js` — private messages

### `initChat()`

Opens `ws://host/ws` (or `wss://` on HTTPS). The `onmessage` handler
dispatches on `msg.type`:

- `"online"` → update the `online` set and re-render the sidebar.
- `"message"` → `onMessage(msg)`.

### `loadUsers()`

Renders the sidebar in two groups — "Recent discussions" (users with an
existing conversation) and "Find friends" (everyone else) — with online dots
and a "new" badge for users in the `unread` set.

### `onMessage(msg)`

- If the message belongs to the **open** chat: append it and scroll down.
- Otherwise (and it's not my own echo): add the sender to `unread` — the
  notification badge, without refreshing.
- Either way, re-render the user list so the sender jumps to the top
  (Discord-style reordering).

### `openChat(id, nickname)`

Loads the last 10 messages. Note the guard:

```js
if (openChatId !== id) return; // user already switched to another chat
```

If you clicked another user while this request was in flight, the stale
response is discarded — **race-condition protection**.

### `loadMore()` — pagination upward

When you scroll to the top of the chat box (throttled listener,
`scrollTop <= 20`), fetch 10 older messages, `prepend` them, and restore the
scroll position with `scrollHeight - oldHeight` math so the view doesn't jump.

### Sending

The submit handler trims the input and sends
`{type: "message", to: openChatId, content}` over the WebSocket. The server
re-validates the session and the content for every message it receives.

### `closeChatEverything()`

Logout cleanup: close the socket, clear `online`/`unread`/`openChatId`, hide
the chat box, empty the sidebar.
