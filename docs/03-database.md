# 3. Database Layer — `internals/database` + `schema.sql`

## `InitDB()` — `internals/database/db.go`

Opens SQLite with a special connection string:

```
./forum.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)
```

- **`foreign_keys(1)`** — makes `ON DELETE CASCADE` work (deleting a post
  automatically deletes its comments, reactions, and category links).
- **`busy_timeout(5000)`** — if two requests write at the same time, SQLite
  waits up to 5 seconds instead of immediately failing with
  "database is locked".
- Putting the pragmas **in the DSN** matters because Go pools connections —
  a plain `PRAGMA` executed once would only apply to one pooled connection;
  the DSN applies them to every connection.

Then it:

1. Reads and executes `schema.sql` — all tables use `CREATE TABLE IF NOT
   EXISTS`, so restarting the app is always safe.
2. Calls `seedCategories()`.

The handle is stored in the package variable `database.DB`, shared by the
whole app.

## `seedCategories()` — `internals/database/seed.go`

Inserts the default categories `"General"`, `"Technology"`, `"Art"`,
`"Science"` using `INSERT OR IGNORE` — a no-op if they already exist, so
rerunning never creates duplicates.

## The tables — `schema.sql`

| Table | Purpose |
|---|---|
| `users` | account data; `nickname` and `email` are UNIQUE; password stored as a bcrypt hash |
| `sessions` | session id (random token) → user id + expiry date |
| `categories` | topic names (UNIQUE) |
| `posts` | title + content + author; CHECK constraints reject empty text |
| `post_categories` | many-to-many link between posts and categories (composite primary key) |
| `comments` | replies to posts |
| `reactions` | likes/dislikes |
| `messages` | private chat messages (sender, receiver, content) |

### Notable constraints

- Every child table has `FOREIGN KEY ... ON DELETE CASCADE` — deleting a user
  removes their sessions, posts, comments, reactions, and messages; deleting a
  post removes its comments, reactions, and category links.
- The `reactions` table has two CHECK constraints:

```sql
type INTEGER NOT NULL CHECK(type IN (1, -1)),      -- like or dislike only

CHECK (                                            -- must target either a post
    (post_id IS NOT NULL AND comment_id IS NULL)   -- OR a comment, never both,
 OR (post_id IS NULL AND comment_id IS NOT NULL)   -- never neither
)
```

- `posts`, `comments`, and `messages` all use
  `CHECK(length(...) > 0)` so empty content can never be stored, even if a
  handler bug let it through.
