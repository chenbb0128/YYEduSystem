-- name: CreateUser :execresult
INSERT INTO users (
    organization_id,
    username,
    password_hash,
    role,
    nickname,
    avatar,
    status
) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetUserByID :one
SELECT
    users.id,
    users.organization_id,
    users.username,
    users.password_hash,
    users.role,
    users.nickname,
    users.avatar,
    users.status,
    users.created_at,
    users.updated_at,
    COALESCE(organizations.status, 'active') AS organization_status
FROM users
LEFT JOIN organizations ON organizations.id = users.organization_id
WHERE users.id = ?
LIMIT 1;

-- name: GetUserByUsername :one
SELECT
    users.id,
    users.organization_id,
    users.username,
    users.password_hash,
    users.role,
    users.nickname,
    users.avatar,
    users.status,
    users.created_at,
    users.updated_at,
    COALESCE(organizations.status, 'active') AS organization_status
FROM users
LEFT JOIN organizations ON organizations.id = users.organization_id
WHERE users.username = ?
LIMIT 1;

-- name: UpdateUserProfile :execresult
UPDATE users
SET
	nickname = ?,
	avatar = ?
WHERE id = ?;

-- name: ListUsers :many
SELECT
    users.id,
    users.organization_id,
    users.username,
    users.password_hash,
    users.role,
    users.nickname,
    users.avatar,
    users.status,
    users.created_at,
    users.updated_at,
    COALESCE(organizations.status, 'active') AS organization_status
FROM users
LEFT JOIN organizations ON organizations.id = users.organization_id
ORDER BY users.id ASC;

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
