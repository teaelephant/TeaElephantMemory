-- Apply explicitly after both 20261012 migrations and a backup. Safe to repeat.
BEGIN;
-- A personal vote belongs to a user and shared Tea, never a QR or collection.
CREATE TABLE IF NOT EXISTS tea_ratings (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tea_id uuid NOT NULL REFERENCES teas(id) ON DELETE CASCADE,
    rating int NOT NULL CHECK (rating BETWEEN 1 AND 5),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (user_id, tea_id)
);
CREATE INDEX IF NOT EXISTS tea_ratings_tea_idx ON tea_ratings(tea_id);
ALTER TABLE tea_merge_receipt_details ADD COLUMN IF NOT EXISTS ratings_moved int NOT NULL DEFAULT 0 CHECK (ratings_moved >= 0);
ALTER TABLE tea_merge_receipt_details ADD COLUMN IF NOT EXISTS rating_conflicts_resolved int NOT NULL DEFAULT 0 CHECK (rating_conflicts_resolved >= 0);
COMMIT;
