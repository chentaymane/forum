CREATE TABLE  users (
    id BLOB PRIMARY KEY NOT NULL,
    email TEXT UNIQUE NOT NULL,
    username TEXT UNIQUE NOT NULL,
    password BLOB NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE sessions (
  id BLOB PRIMARY KEY,
  user_id BLOB NOT NULL, 
  FOREIGN KEY(user_id) REFERENCES users(id)
);
