-- ═══════════════════════════════════════════════════════════════════════════
--  Forum Database Schema (SQLite)
-- ═══════════════════════════════════════════════════════════════════════════
--
-- SQLite is used because it requires zero configuration – the database is
-- just a single file (forum.db).  Every table uses INTEGER PRIMARY KEY for
-- compact, fast row IDs.
--
-- ─── Tables ──────────────────────────────────────────────────────────────
--   users            Registered accounts (nickname, age, gender, …)
--   sessions         Active login sessions (one per user)
--   posts            Forum posts
--   categories       Available post categories
--   post_categories  Many‑to‑many link between posts and categories
--   comments         Replies on posts
--   reactions        Likes / dislikes on posts AND comments
--   private_messages Person‑to‑person chat messages
--

-- ─── Users ────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL UNIQUE,
    nickname TEXT,
    age INTEGER,
    gender TEXT,
    first_name TEXT,
    last_name TEXT,
    password TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ─── Posts ────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    title VARCHAR(60) NOT NULL CHECK(length(title) > 0),
    content TEXT NOT NULL CHECK(length(content) > 0),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- ─── Categories ────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

-- ─── Post ↔ Categories (many‑to‑many) ─────────────────────────────────────
CREATE TABLE IF NOT EXISTS post_categories (
    post_id INTEGER NOT NULL,
    category_id INTEGER NOT NULL,
    PRIMARY KEY (post_id, category_id),
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
);

-- ─── Comments ──────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    content TEXT NOT NULL CHECK(length(content) > 0),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- ─── Reactions (likes / dislikes) ─────────────────────────────────────────
-- type: 1  = like
--       -1 = dislike
-- Each reaction targets EITHER a post OR a comment (enforced by CHECK).
CREATE TABLE IF NOT EXISTS reactions (
    user_id INTEGER NOT NULL,
    post_id INTEGER,
    comment_id INTEGER,
    type INTEGER NOT NULL CHECK(type IN (1, -1)),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY (comment_id) REFERENCES comments(id) ON DELETE CASCADE,
    CHECK (
        (post_id IS NOT NULL AND comment_id IS NULL) OR
        (post_id IS NULL AND comment_id IS NOT NULL)
    )
);

-- ─── Sessions ──────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL,
    expires_at DATETIME NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- ─── Private Messages ──────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS private_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sender_id INTEGER NOT NULL,
    receiver_id INTEGER NOT NULL,
    content TEXT NOT NULL CHECK(length(content) > 0),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (sender_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (receiver_id) REFERENCES users(id) ON DELETE CASCADE
);

-- ═══ Indexes ═══════════════════════════════════════════════════════════════
-- Speeds up the most common queries: loading a conversation between two
-- users, and filtering messages by receiver.

CREATE INDEX IF NOT EXISTS idx_private_messages_pair_time
    ON private_messages(sender_id, receiver_id, created_at);

CREATE INDEX IF NOT EXISTS idx_private_messages_receiver_time
    ON private_messages(receiver_id, created_at);
