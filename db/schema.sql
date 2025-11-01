-- PostgreSQL schema aligned with docs/FDB_TO_POSTGRES_MIGRATION_PLAN.md
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
