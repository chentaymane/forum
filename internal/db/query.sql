-- name: CreateUser :exec
INSERT INTO users ( id, username, email, password, created_at)
VALUES ( ?, ?, ?, ?, ? );
-- name: GetUser :one
SELECT * FROM users 
WHERE email = ?;
-- name: CreateSession :one
INSERT INTO sessions (id, user_id, created_at)
VALUES (?, ?, ? )
RETURNING id;
-- name: GetSession :one
SELECT sessions.id, users.id as user_id , users.email, users.username
FROM sessions
JOIN users ON sessions.user_id = users.id
WHERE sessions.id = ?;
-- name: DeleteUserSession :exec
DELETE FROM sessions 
WHERE id = ?;
-- name: CreatePost :one
INSERT INTO posts (id, title, content, user_id, created_at) 
VALUES (?, ?, ?, ?, ? )
RETURNING title;
-- name: SaveImage :exec 
INSERT INTO images (id, post_id, created_at)
VALUES (?, ?, ?);
-- name: GetPosts :many 
SELECT * FROM posts  
WHERE created_at < ?
ORDER BY craeted_at DESC
LIMIT 20;
