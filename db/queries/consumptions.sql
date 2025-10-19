-- name: InsertConsumption :exec
INSERT INTO consumptions (user_id, ts, tea_id)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, ts, tea_id) DO NOTHING;

-- name: DeleteConsumptionsBefore :exec
DELETE FROM consumptions
WHERE user_id = $1 AND ts < $2;

-- name: ListConsumptionsSince :many
SELECT ts, tea_id
FROM consumptions
WHERE user_id = $1 AND ts >= $2
ORDER BY ts DESC;
