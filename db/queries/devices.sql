-- name: InsertDevice :exec
INSERT INTO devices (id, user_id, token)
VALUES ($1, $2, $3)
ON CONFLICT (id) DO NOTHING;

-- name: UpdateDeviceToken :exec
UPDATE devices
SET token = $2
WHERE id = $1;

-- name: ListDeviceTokens :many
SELECT token
FROM devices
WHERE user_id = $1
ORDER BY created_at DESC;
