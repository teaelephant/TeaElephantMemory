-- name: UpsertUser :one
INSERT INTO users (id, apple_id)
VALUES ($1, $2)
ON CONFLICT (apple_id)
DO UPDATE SET apple_id = EXCLUDED.apple_id
RETURNING id, apple_id, created_at;

-- name: ListUsers :many
SELECT id, apple_id, created_at
FROM users
ORDER BY created_at DESC;
