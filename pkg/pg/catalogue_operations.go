package pg

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/teaelephant/TeaElephantMemory/common"
)

// Small household catalogs serialize writes before locking individual entities.
// This also orders taxonomy cascades against parent Tea/image/tag writes.
const catalogueWriteLock int64 = 724383810271
const catalogTeaTable = "teas"
const catalogTagTable = "tags"
const catalogCategoryTable = "tag_categories"
const catalogNewTeaOperation = "newTea"
const catalogAddImageOperation = "addTeaImage"

func beginCatalogueWrite(ctx context.Context, connection *sql.DB) (*sql.Tx, error) {
	tx, err := connection.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin catalog mutation: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, catalogueWriteLock); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("lock catalog mutation: %w", err)
	}

	return tx, nil
}

func checkCatalogueRevision(ctx context.Context, tx *sql.Tx, table string, id uuid.UUID) error {
	var actual string
	// table is selected only from literals at internal call sites, never request input.
	err := tx.QueryRowContext(ctx, `SELECT revision::text FROM `+table+` WHERE id=$1 FOR UPDATE`, id).Scan(&actual)
	if errors.Is(err, sql.ErrNoRows) {
		return common.ErrTeaNotFound
	}

	if err != nil {
		return fmt.Errorf("read catalog revision: %w", err)
	}

	option := common.CatalogueOperationFrom(ctx)
	if option.HasExpectedRevision && actual != option.ExpectedRevision {
		return &common.CatalogueConflictError{CurrentRevision: actual}
	}

	return nil
}

func (d *db) withCatalogueMutation(ctx context.Context, table string, id uuid.UUID, update func(*sql.Tx) error) error {
	tx, err := beginCatalogueWrite(ctx, d.pg)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err = checkCatalogueRevision(ctx, tx, table, id); err != nil {
		return err
	}

	if err = update(tx); err != nil {
		return err
	}

	return wrapCatalogStorageError(tx.Commit())
}

type rowScanner interface {
	Scan(destinations ...any) error
}

func scanCatalogueTea(row rowScanner) (*common.Tea, error) {
	record := &common.Tea{TeaData: &common.TeaData{}}

	var kind string

	err := row.Scan(&record.ID, &record.Name, &kind, &record.Description, &record.Revision)
	record.Type = common.StringToBeverageType(kind)

	return record, wrapCatalogStorageError(err)
}

func operationHash(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", wrapCatalogStorageError(err)
	}

	digest := sha256.Sum256(payload)

	return hex.EncodeToString(digest[:]), nil
}

func readReceipt(ctx context.Context, query interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, key uuid.UUID) (*common.CatalogueReceipt, string, error) {
	receipt := &common.CatalogueReceipt{}

	var hash string

	err := query.QueryRowContext(ctx, `SELECT operation_key,actor,operation,request_hash,tea_id,resource_id,result_revision,completed_at FROM catalog_operation_receipts WHERE operation_key=$1`, key).
		Scan(&receipt.Key, &receipt.Actor, &receipt.Operation, &hash, &receipt.TeaID, &receipt.ResourceID, &receipt.ResultRevision, &receipt.CompletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", nil
	}

	if err != nil {
		return nil, "", fmt.Errorf("read catalog receipt: %w", err)
	}

	return receipt, hash, nil
}

//nolint:nilnil // Calls without an idempotency key have no replay receipt.
func replayReceipt(ctx context.Context, tx *sql.Tx, operation, hash string) (*common.CatalogueReceipt, error) {
	options := common.CatalogueOperationFrom(ctx)
	if !options.HasKey {
		return nil, nil
	}

	receipt, storedHash, err := readReceipt(ctx, tx, options.Key)
	if err != nil || receipt == nil {
		return receipt, err
	}

	if receipt.Actor != options.Actor || receipt.Operation != operation || storedHash != hash {
		return nil, common.ErrIdempotencyConflict
	}

	return receipt, nil
}

func saveReceipt(ctx context.Context, tx *sql.Tx, operation, hash string, teaID, resourceID uuid.UUID) error {
	options := common.CatalogueOperationFrom(ctx)
	if !options.HasKey {
		return nil
	}

	_, err := tx.ExecContext(ctx, `INSERT INTO catalog_operation_receipts(operation_key,actor,operation,request_hash,tea_id,resource_id,result_revision) SELECT $1,$2,$3,$4,$5,$6,revision::text FROM teas WHERE id=$5`, options.Key, options.Actor, operation, hash, teaID, resourceID)
	if err != nil {
		return fmt.Errorf("save catalog receipt: %w", err)
	}

	return nil
}

// CatalogueOperationReceipt returns committed receipts; permission checks belong to the API boundary.
func (d *db) CatalogueOperationReceipt(ctx context.Context, key uuid.UUID) (*common.CatalogueReceipt, error) {
	receipt, _, err := readReceipt(ctx, d.pg, key)
	return receipt, err
}

// CatalogueCapabilities reports whether the explicit migration prerequisite is installed.
func (d *db) CatalogueCapabilities(ctx context.Context) (bool, error) {
	var ready bool

	err := d.pg.QueryRowContext(ctx, `SELECT to_regclass(current_schema() || '.catalog_operation_receipts') IS NOT NULL AND (SELECT count(*)=3 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name IN ('teas','tags','tag_categories') AND column_name='revision' AND data_type='bigint' AND is_nullable='NO') AND (SELECT count(*)=5 FROM pg_trigger WHERE NOT tgisinternal AND tgenabled <> 'D' AND tgname IN ('catalog_tea_revision','catalog_tag_revision','catalog_category_revision','catalog_tea_tags_revision','catalog_tea_images_revision') AND tgrelid IN (to_regclass(current_schema() || '.teas'),to_regclass(current_schema() || '.tags'),to_regclass(current_schema() || '.tag_categories'),to_regclass(current_schema() || '.tea_tags'),to_regclass(current_schema() || '.tea_images')))`).Scan(&ready)

	return ready, wrapCatalogStorageError(err)
}

func wrapCatalogStorageError(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("catalog storage: %w", err)
}
