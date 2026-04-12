-- USERS
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL,
    username TEXT NOT NULL,
    password TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(email),
    UNIQUE(username)
);

-- POSTS
CREATE TABLE IF NOT EXISTS posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    title TEXT NOT NULL CHECK(length(title) > 0),
    content TEXT NOT NULL CHECK(length(content) > 0),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- COMMENTS
CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    content TEXT NOT NULL CHECK(length(content) > 0),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- CATEGORIES
CREATE TABLE IF NOT EXISTS categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

-- POST <-> CATEGORY (MANY-TO-MANY)
CREATE TABLE IF NOT EXISTS post_categories (
    post_id INTEGER NOT NULL,
    category_id INTEGER NOT NULL,

    PRIMARY KEY (post_id, category_id),

    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
);

-- LIKES / DISLIKES
CREATE TABLE IF NOT EXISTS reactions (
    user_id INTEGER NOT NULL,
    post_id INTEGER,
    comment_id INTEGER,
    type INTEGER NOT NULL CHECK(type IN (1, -1)),

    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY (comment_id) REFERENCES comments(id) ON DELETE CASCADE,

    -- MUST target either post OR comment
    CHECK (
        (post_id IS NOT NULL AND comment_id IS NULL) OR
        (post_id IS NULL AND comment_id IS NOT NULL)
    )
);

-- SESSIONS
DROP TABLE IF EXISTS sessions;

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL,
    expires_at DATETIME NOT NULL,

    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- =========================
-- 🔥 PERFORMANCE INDEXES
-- =========================

-- POSTS
CREATE INDEX IF NOT EXISTS idx_posts_user_created 
ON posts(user_id, created_at DESC);

-- COMMENTS
CREATE INDEX IF NOT EXISTS idx_comments_post_created 
ON comments(post_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_comments_user 
ON comments(user_id);

-- CATEGORY LOOKUPS
CREATE INDEX IF NOT EXISTS idx_post_categories_post 
ON post_categories(post_id);

CREATE INDEX IF NOT EXISTS idx_post_categories_category 
ON post_categories(category_id);

-- LIKES
CREATE INDEX IF NOT EXISTS idx_likes_post 
ON reactions(post_id);

CREATE INDEX IF NOT EXISTS idx_likes_comment 
ON reactions(comment_id);

-- SESSIONS
CREATE INDEX IF NOT EXISTS idx_sessions_user 
ON sessions(user_id);

CREATE INDEX IF NOT EXISTS idx_sessions_expiry 
ON sessions(expires_at);

-- =========================
-- 🔒 UNIQUE REACTIONS
-- =========================

CREATE UNIQUE INDEX IF NOT EXISTS unique_post_reaction
ON reactions(user_id, post_id)
WHERE comment_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS unique_comment_reaction
ON reactions(user_id, comment_id)
WHERE post_id IS NULL;

-- =========================
-- ⚡ TRIGGERS (TOGGLE LOGIC)
-- =========================

CREATE TRIGGER IF NOT EXISTS toggle_post_reaction
BEFORE INSERT ON reactions
FOR EACH ROW
WHEN NEW.comment_id IS NULL
BEGIN
    -- same reaction → remove + cancel insert
    DELETE FROM reactions
    WHERE user_id = NEW.user_id
    AND post_id = NEW.post_id
    AND comment_id IS NULL
    AND type = NEW.type;

    SELECT RAISE(IGNORE)
    WHERE changes() > 0;

    -- opposite reaction → replace
    DELETE FROM reactions
    WHERE user_id = NEW.user_id
    AND post_id = NEW.post_id
    AND comment_id IS NULL;
END;

CREATE TRIGGER IF NOT EXISTS toggle_comment_reaction
BEFORE INSERT ON reactions
FOR EACH ROW
WHEN NEW.post_id IS NULL
BEGIN
    DELETE FROM reactions
    WHERE user_id = NEW.user_id
    AND comment_id = NEW.comment_id
    AND post_id IS NULL
    AND type = NEW.type;

    SELECT RAISE(IGNORE)
    WHERE changes() > 0;

    DELETE FROM reactions
    WHERE user_id = NEW.user_id
    AND comment_id = NEW.comment_id
    AND post_id IS NULL;
END;
