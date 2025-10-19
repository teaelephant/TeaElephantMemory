-- name: ListNotifications :many
SELECT id, user_id, type, created_at
FROM notifications
WHERE user_id = $1
ORDER BY created_at DESC;
