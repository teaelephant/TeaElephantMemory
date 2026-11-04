-- PostgreSQL schema documented in docs/POSTGRES_STORAGE.md
-- This file is used by sqlc for type generation and can also be used as a base migration.

CREATE TABLE IF NOT EXISTS users (
  id uuid PRIMARY KEY,
  apple_id text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS teas (
  id uuid PRIMARY KEY,
  name text NOT NULL,
  type text NOT NULL CHECK (type IN ('tea','herb','coffee','other')),
  description text,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS teas_name_prefix_idx ON teas (lower(name) text_pattern_ops);

CREATE TABLE IF NOT EXISTS tag_categories (
  id uuid PRIMARY KEY,
  name text NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS tags (
  id uuid PRIMARY KEY,
  name text NOT NULL,
  color text NOT NULL,
  category_id uuid NOT NULL REFERENCES tag_categories(id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX IF NOT EXISTS tags_category_name_uq ON tags (category_id, lower(name));
CREATE INDEX IF NOT EXISTS tags_category_idx ON tags (category_id);

CREATE TABLE IF NOT EXISTS tea_tags (
  tea_id uuid NOT NULL REFERENCES teas(id) ON DELETE CASCADE,
  tag_id uuid NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (tea_id, tag_id)
);
CREATE INDEX IF NOT EXISTS tea_tags_tag_idx ON tea_tags (tag_id);

CREATE TABLE IF NOT EXISTS qr_records (
  id uuid PRIMARY KEY,
  tea_id uuid NOT NULL REFERENCES teas(id) ON DELETE CASCADE,
  boiling_temp int NOT NULL,
  expiration_date timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS qr_records_tea_idx ON qr_records (tea_id);
CREATE INDEX IF NOT EXISTS qr_records_exp_idx ON qr_records (expiration_date);
-- Likely filter criterion during brewing suggestions/search
CREATE INDEX IF NOT EXISTS qr_records_boiling_temp_idx ON qr_records (boiling_temp);

CREATE TABLE IF NOT EXISTS collections (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS collections_user_idx ON collections (user_id);

CREATE TABLE IF NOT EXISTS collection_qr_items (
  collection_id uuid NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  qr_id uuid NOT NULL REFERENCES qr_records(id) ON DELETE CASCADE,
  PRIMARY KEY (collection_id, qr_id)
);
CREATE INDEX IF NOT EXISTS collection_qr_items_qr_idx ON collection_qr_items (qr_id);

CREATE TABLE IF NOT EXISTS devices (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (token)
);
CREATE INDEX IF NOT EXISTS devices_user_idx ON devices (user_id);

CREATE TABLE IF NOT EXISTS notifications (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type smallint NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS notifications_user_created_idx ON notifications (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS consumptions (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  ts timestamptz NOT NULL,
  tea_id uuid NOT NULL REFERENCES teas(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, ts, tea_id)
);
CREATE INDEX IF NOT EXISTS consumptions_user_ts_desc_idx ON consumptions (user_id, ts DESC);
-- Apply explicitly to an existing database after a backup; safe to run again.
CREATE TABLE IF NOT EXISTS tea_images (
  id uuid PRIMARY KEY,
  tea_id uuid NOT NULL REFERENCES teas(id) ON DELETE CASCADE,
  content_type text NOT NULL CHECK (content_type IN ('image/jpeg', 'image/png')),
  width int NOT NULL CHECK (width > 0 AND width <= 8192),
  height int NOT NULL CHECK (height > 0 AND height <= 8192),
  data bytea NOT NULL CHECK (octet_length(data) > 0 AND octet_length(data) <= 5242880),
  created_at timestamptz NOT NULL DEFAULT now(),
  is_cover boolean NOT NULL DEFAULT false,
  CHECK (width::bigint * height::bigint <= 16000000)
);
CREATE INDEX IF NOT EXISTS tea_images_tea_created_idx ON tea_images (tea_id, created_at, id);
CREATE UNIQUE INDEX IF NOT EXISTS tea_images_one_cover_idx ON tea_images (tea_id) WHERE is_cover;
-- Apply explicitly after a backup. Additive and safe to apply again.
ALTER TABLE teas ADD COLUMN IF NOT EXISTS revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0);
ALTER TABLE tags ADD COLUMN IF NOT EXISTS revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0);
ALTER TABLE tag_categories ADD COLUMN IF NOT EXISTS revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0);

CREATE TABLE IF NOT EXISTS catalog_operation_receipts (
    operation_key uuid PRIMARY KEY,
    actor text NOT NULL,
    operation text NOT NULL CHECK (operation IN ('newTea','addTeaImage','mergeTeas')),
    request_hash text NOT NULL,
    tea_id uuid NOT NULL,
    resource_id uuid NOT NULL,
    result_revision text NOT NULL,
    completed_at timestamptz NOT NULL DEFAULT now()
);

CREATE OR REPLACE FUNCTION catalog_bump_revision() RETURNS trigger AS $$
BEGIN
    NEW.revision := OLD.revision + 1;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION catalog_bump_parent_tea_revision() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        UPDATE teas SET revision = revision + 1 WHERE id = OLD.tea_id;
        RETURN OLD;
    ELSE
        IF TG_OP = 'UPDATE' THEN
            IF OLD.tea_id IS DISTINCT FROM NEW.tea_id THEN
                UPDATE teas SET revision = revision + 1 WHERE id = OLD.tea_id;
            END IF;
        END IF;
        UPDATE teas SET revision = revision + 1 WHERE id = NEW.tea_id;
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS catalog_tea_revision ON teas;
CREATE TRIGGER catalog_tea_revision BEFORE UPDATE ON teas FOR EACH ROW EXECUTE FUNCTION catalog_bump_revision();
DROP TRIGGER IF EXISTS catalog_tag_revision ON tags;
CREATE TRIGGER catalog_tag_revision BEFORE UPDATE ON tags FOR EACH ROW EXECUTE FUNCTION catalog_bump_revision();
DROP TRIGGER IF EXISTS catalog_category_revision ON tag_categories;
CREATE TRIGGER catalog_category_revision BEFORE UPDATE ON tag_categories FOR EACH ROW EXECUTE FUNCTION catalog_bump_revision();
DROP TRIGGER IF EXISTS catalog_tea_tags_revision ON tea_tags;
CREATE TRIGGER catalog_tea_tags_revision AFTER INSERT OR UPDATE OR DELETE ON tea_tags FOR EACH ROW EXECUTE FUNCTION catalog_bump_parent_tea_revision();
DROP TRIGGER IF EXISTS catalog_tea_images_revision ON tea_images;
CREATE TRIGGER catalog_tea_images_revision AFTER INSERT OR UPDATE OR DELETE ON tea_images FOR EACH ROW EXECUTE FUNCTION catalog_bump_parent_tea_revision();

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
