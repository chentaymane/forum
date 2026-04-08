-- name: CreateUser :exec
INSERT INTO users ( id, username, email, password)
VALUES ( ?, ?, ?, ?);
-- name: GetUser :one
SELECT * FROM users 
WHERE username = ?;
-- name: CreateSession :one
INSERT INTO sessions (id, user_id)
VALUES (?, ?)
RETURNING id;
-- name: GetSession :one
SELECT * FROM sessions
WHERE id = ?;
