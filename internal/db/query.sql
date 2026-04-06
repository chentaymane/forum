--name: CreateUser :one
INSERT INTO users ( id, username, password)
VALUES ( $1, $2, $3);
--name: GetUser :one
SELECT * FROM users 
WHERE username = $1 LIMIT $1;

