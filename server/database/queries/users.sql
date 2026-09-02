-- name: CreateUser :execresult
INSERT INTO users (
    username,
    password_hash,
    role,
    nickname,
    avatar,
    status
) VALUES (?, ?, ?, ?, ?, ?);

-- name: GetUserByID :one
SELECT
    id,
    username,
    password_hash,
    role,
    nickname,
    avatar,
    status,
    created_at,
    updated_at
FROM users
WHERE id = ?
LIMIT 1;

-- name: GetUserByUsername :one
SELECT
    id,
    username,
    password_hash,
    role,
    nickname,
    avatar,
    status,
    created_at,
    updated_at
FROM users
WHERE username = ?
LIMIT 1;

-- name: UpdateUserProfile :execresult
UPDATE users
SET
	nickname = ?,
	avatar = ?
WHERE id = ?;

-- name: ListUsers :many
SELECT
    id,
    username,
    password_hash,
    role,
    nickname,
    avatar,
    status,
    created_at,
    updated_at
FROM users
ORDER BY id ASC;

-- name: UpdateUser :execresult
UPDATE users
SET
	password_hash = ?,
	role = ?,
	nickname = ?,
	avatar = ?,
	status = ?
WHERE id = ?;

-- name: SetUserStatus :execresult
UPDATE users
SET status = ?
WHERE id = ?;
