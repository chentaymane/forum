CREATE TABLE IF NOT EXISTS users (
    id BLOB PRIMARY KEY NOT NULL,
    email TEXT UNIQUE NOT NULL,
    username TEXT UNIQUE NOT NULL,
    password BLOB NOT NULL,
    created_at INTEGER DEFAULT (unixepoch())
);
CREATE TABLE  IF NOT EXISTS sessions (
  id BLOB NOT NULL PRIMARY KEY,
  created_at INTEGER  DEFAULT (unixepoch()),
  user_id BLOB NOT NULL, 
  FOREIGN KEY(user_id) REFERENCES users(id)
);
CREATE TABLE IF NOT EXISTS posts (
  id BLOB NOT NULL,
  title TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at INTEGER  DEFAULT (unixepoch()),
  user_id BLOB NOT NULL,
  FOREIGN KEY(user_id) REFERENCES users(id)
);
CREATE TABLE IF NOT EXISTS images (
  id BLOB NOT NULL PRIMARY KEY,
  created_at INTEGER  DEFAULT (unixepoch()),
  post_id BLOB NOT NULL,
  FOREIGN KEY(post_id) REFERENCES  posts(id)
);
