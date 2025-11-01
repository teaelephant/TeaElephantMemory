-- name: InsertCollection :one
INSERT INTO collections (id, user_id, name)
VALUES ($1, $2, $3)
RETURNING id, user_id, name, created_at;

-- name: InsertCollectionItem :exec
INSERT INTO collection_qr_items (collection_id, qr_id)
VALUES ($1, $2)
ON CONFLICT (collection_id, qr_id) DO NOTHING;

-- name: DeleteCollectionItem :exec
DELETE FROM collection_qr_items
WHERE collection_id = $1 AND qr_id = $2;

-- name: DeleteCollection :exec
DELETE FROM collections
WHERE id = $1 AND user_id = $2;

-- name: ListCollections :many
SELECT id, user_id, name, created_at
FROM collections
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: GetCollection :one
SELECT id, user_id, name, created_at
FROM collections
WHERE id = $1 AND user_id = $2;

-- name: ListCollectionRecords :many
SELECT
  q.id AS qr_id,
  t.id AS tea_id,
  t.name,
  t.type,
  t.description,
  q.boiling_temp,
  q.expiration_date
FROM collection_qr_items c
JOIN qr_records q ON q.id = c.qr_id
JOIN teas t ON t.id = q.tea_id
WHERE c.collection_id = $1
ORDER BY q.expiration_date ASC;

-- name: InsertCollectionItems :exec
INSERT INTO collection_qr_items (collection_id, qr_id)
SELECT $1, x
FROM unnest($2::uuid[]) AS t(x)
JOIN qr_records q ON q.id = x
ON CONFLICT (collection_id, qr_id) DO NOTHING;
