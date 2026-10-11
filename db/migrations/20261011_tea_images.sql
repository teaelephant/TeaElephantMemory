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
