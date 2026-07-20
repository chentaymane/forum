# 5. Forum Package — `internals/forum`

## Posts — `posts.go`

### `PostsHandler` — `GET`/`POST /api/posts`

Routes by HTTP method. One trick to understand: since `http.HandleFunc` only
gives one URL per path, **delete is done via POST** with
`{"action": "delete", "id": 5}`. The handler reads the body once, "probes" it
for the `action` field, and dispatches to `deletePost` or
`createPostFromBody`. The already-read body is passed along, because a request
body can only be read once.

### `getPosts` — the feed query

The biggest query in the app. One SQL statement returns, *per post*:

- author nickname (`JOIN users`)
- date, trimmed to `YYYY-MM-DD HH:MM` with `substr(created_at, 1, 16)`
- comma-joined category names (a `GROUP_CONCAT` subquery)
- like count and dislike count (two `COUNT(*)` subqueries)
- how **the current user** reacted: `1`, `-1`, or `0` (a `COALESCE` subquery)
- comment count

Then it appends WHERE conditions based on query parameters:

| Parameter | Filter |
|---|---|
| `?category=2` | posts linked to that category |
| `?mine=1` | posts written by the current user |
| `?commented=1` | posts the current user commented on |
| `?liked=1` | posts the current user liked |

Paging: `ORDER BY p.id DESC LIMIT 10 OFFSET ?` — newest first, 10 at a time,
which feeds the infinite scroll. `?offset=<n>` is how many posts the client
already loaded.

Everything uses `?` placeholders → **no SQL injection**.

### `createPostFromBody`

1. Validates title/content: non-empty after trimming, title ≤ 100 chars,
   content ≤ 2000 chars.
2. Inserts the post.
3. Links categories — with two safety steps:
   - It loads the set of **valid** category ids from the DB first and ignores
     any bogus ids the client sends (also deduplicates).
   - If no valid category was chosen, the post is auto-filed under
     **"General"**.

### `deletePost`

The ownership-check pattern (used for comments too):

1. `SELECT user_id FROM posts WHERE id = ?` → no row = `404 post not found`.
2. Owner ≠ requester = `403 you can only delete your own posts`.
3. Otherwise DELETE — and the `ON DELETE CASCADE` foreign keys remove its
   comments, reactions, and category links automatically.

## Comments — `comments.go`

Mirror image of posts:

- **`CommentsHandler`** — same method routing and action-probe dispatch.
- **`getComments`** — all comments of `?post_id=<id>`, oldest first, each with
  like/dislike counts and the current user's reaction (same subquery pattern
  as `getPosts`).
- **`createCommentFromBody`** — validate (non-empty, ≤ 1000 chars) → insert.
- **`deleteComment`** — same ownership check as `deletePost`.

## Reactions — `reactions.go`

### `ReactionsHandler` — `POST /api/reactions`

Implements the like/dislike toggle logic in one function:

1. **Validate**: `type` must be `1` or `-1`, and *exactly one* of
   `postId`/`commentId` must be set. The clever line

   ```go
   (in.PostID > 0) == (in.CommentID > 0)
   ```

   rejects both-set and neither-set — it acts as an exclusive-or (XOR).

2. **Pick the column**: `post_id` or `comment_id`. The column name is
   concatenated into the SQL string, but that's safe because it's chosen from
   two hardcoded values, never user text.

3. **Toggle logic**: delete the user's existing reaction on this target; then
   re-insert the new one *unless* the user clicked the same button again:
   - like → like = remove the like (toggle off)
   - like → dislike = replace

4. **Return fresh counts** (`likes`, `dislikes`, `reactedTo`) so the frontend
   updates the buttons without reloading the page.

## Categories — `categories.go`

**`GetCategories`** — returns all categories ordered by id. Used to fill the
"new post" checkbox list and the filter pills.

## Models — `models.go`

Defines the `Post`, `Comment`, and `Category` structs with `json:"..."` tags
that shape the API responses. Notable field: `ReactedTo` is `1`, `-1`, or `0`
for the logged-in user, so the UI can highlight the button they pressed.
