-- name: UpsertQR :exec
INSERT INTO qr_records (id, tea_id, boiling_temp, expiration_date)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE
SET tea_id = EXCLUDED.tea_id,
    boiling_temp = EXCLUDED.boiling_temp,
    expiration_date = EXCLUDED.expiration_date;

-- name: GetQR :one
SELECT id, tea_id, boiling_temp, expiration_date, created_at
FROM qr_records
WHERE id = $1;
