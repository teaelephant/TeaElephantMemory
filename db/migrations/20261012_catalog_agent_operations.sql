-- Apply explicitly after a backup. Additive and safe to apply again.
ALTER TABLE teas ADD COLUMN IF NOT EXISTS revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0);
ALTER TABLE tags ADD COLUMN IF NOT EXISTS revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0);
ALTER TABLE tag_categories ADD COLUMN IF NOT EXISTS revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0);

CREATE TABLE IF NOT EXISTS catalog_operation_receipts (
    operation_key uuid PRIMARY KEY,
    actor text NOT NULL,
    operation text NOT NULL CHECK (operation IN ('newTea','addTeaImage')),
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
