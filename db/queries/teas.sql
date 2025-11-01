-- name: InsertTea :one
INSERT INTO teas (id, name, type, description)
VALUES ($1, $2, $3, $4)
RETURNING id, name, type, description, created_at;

-- name: UpdateTea :one
UPDATE teas
SET name = $2,
    type = $3,
    description = $4
WHERE id = $1
RETURNING id, name, type, description, created_at;

-- name: DeleteTea :exec
DELETE FROM teas WHERE id = $1;

-- name: GetTea :one
SELECT id, name, type, description, created_at
FROM teas
WHERE id = $1;

-- name: ListTeas :many
SELECT id, name, type, description, created_at
FROM teas
ORDER BY created_at DESC;

-- name: SearchTeasByPrefix :many
SELECT id, name, type, description, created_at
FROM teas
WHERE lower(name) LIKE lower($1) || '%'
ORDER BY name ASC
LIMIT $2;
