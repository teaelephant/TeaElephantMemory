-- name: InsertTagCategory :one
INSERT INTO tag_categories (id, name)
VALUES ($1, $2)
RETURNING id, name;

-- name: UpdateTagCategory :one
UPDATE tag_categories
SET name = $2
WHERE id = $1
RETURNING id, name;

-- name: DeleteTagCategory :exec
DELETE FROM tag_categories WHERE id = $1;

-- name: ListTagCategories :many
SELECT id, name
FROM tag_categories
ORDER BY name ASC;

-- name: GetTagCategory :one
SELECT id, name
FROM tag_categories
WHERE id = $1;

-- name: SearchTagCategories :many
SELECT id, name
FROM tag_categories
WHERE lower(name) LIKE lower($1) || '%'
ORDER BY name ASC;

-- name: ListTagsByCategory :many
SELECT id, name, color, category_id
FROM tags
WHERE category_id = $1
ORDER BY name ASC;

-- name: DeleteTagsByCategory :exec
DELETE FROM tags
WHERE category_id = $1;

-- name: InsertTag :one
INSERT INTO tags (id, name, color, category_id)
VALUES ($1, $2, $3, $4)
RETURNING id, name, color, category_id;

-- name: UpdateTag :one
UPDATE tags
SET name = $2,
    color = $3
WHERE id = $1
RETURNING id, name, color, category_id;

-- name: ChangeTagCategory :one
UPDATE tags
SET category_id = $2
WHERE id = $1
RETURNING id, name, color, category_id;

-- name: DeleteTag :exec
DELETE FROM tags WHERE id = $1;

-- name: GetTag :one
SELECT id, name, color, category_id
FROM tags
WHERE id = $1;

-- name: ListTags :many
SELECT id, name, color, category_id
FROM tags
ORDER BY name ASC;

-- name: ListTagsByName :many
SELECT id, name, color, category_id
FROM tags
WHERE lower(name) LIKE lower($1) || '%'
ORDER BY name ASC;

-- name: ListTagsByCategoryFilter :many
SELECT id, name, color, category_id
FROM tags
WHERE category_id = $1
ORDER BY name ASC;

-- name: ListTagsByNameCategory :many
SELECT id, name, color, category_id
FROM tags
WHERE lower(name) LIKE lower($1) || '%'
  AND category_id = $2
ORDER BY name ASC;

-- name: AddTagToTea :exec
INSERT INTO tea_tags (tea_id, tag_id)
VALUES ($1, $2)
ON CONFLICT (tea_id, tag_id) DO NOTHING;

-- name: DeleteTagFromTea :exec
DELETE FROM tea_tags WHERE tea_id = $1 AND tag_id = $2;

-- name: ListTagsByTea :many
SELECT t.id, t.name, t.color, t.category_id
FROM tea_tags tt
JOIN tags t ON t.id = tt.tag_id
WHERE tt.tea_id = $1
ORDER BY t.name ASC;
