-- Apply explicitly after the catalogue operation migration and a PostgreSQL backup.
-- No Tea/QR/image/history IDs or historical receipts are rewritten by this migration.
BEGIN;
ALTER TABLE catalog_operation_receipts DROP CONSTRAINT IF EXISTS catalog_operation_receipts_operation_check;
ALTER TABLE catalog_operation_receipts ADD CONSTRAINT catalog_operation_receipts_operation_check
    CHECK (operation IN ('newTea','addTeaImage','mergeTeas'));
CREATE TABLE IF NOT EXISTS tea_merge_receipt_details (
    operation_key uuid PRIMARY KEY REFERENCES catalog_operation_receipts(operation_key) ON DELETE RESTRICT,
    source_revision text NOT NULL,
    target_revision text NOT NULL,
    preview_token text NOT NULL,
    qr_count int NOT NULL CHECK (qr_count >= 0),
    collection_membership_count int NOT NULL CHECK (collection_membership_count >= 0),
    tags_added int NOT NULL CHECK (tags_added >= 0),
    images_moved int NOT NULL CHECK (images_moved >= 0),
    consumptions_moved int NOT NULL CHECK (consumptions_moved >= 0),
    source_metadata jsonb NOT NULL,
    target_metadata jsonb NOT NULL
);
COMMIT;
