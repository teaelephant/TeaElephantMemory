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

const catalogDeleteTeaSQL = `DELETE FROM teas WHERE id=$1`
const catalogMergeOperation = "mergeTeas"
const teaMergeTableLocks = `LOCK TABLE collection_qr_items, consumptions, qr_records IN SHARE ROW EXCLUSIVE MODE`

// beginTeaMerge orders all catalog writers before dependency tables, then Tea rows.
// Legacy QR/history/membership writers finish before these locks are acquired;
// unlike acquiring Tea rows first, this cannot cycle with their foreign-key locks.
func (d *db) beginTeaMerge(ctx context.Context) (*sql.Tx, error) {
	tx, err := beginCatalogueWrite(ctx, d.pg)
	if err != nil {
		return nil, err
	}

	if _, err = tx.ExecContext(ctx, teaMergeTableLocks); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("lock Tea merge dependencies: %w", err)
	}

	return tx, nil
}

func (d *db) TeaMergeCapabilities(ctx context.Context) (bool, error) {
	var ready bool

	err := d.pg.QueryRowContext(ctx, `SELECT to_regclass(current_schema() || '.tea_merge_receipt_details') IS NOT NULL AND to_regclass(current_schema() || '.tea_ratings') IS NOT NULL AND (SELECT count(*)=2 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='tea_merge_receipt_details' AND column_name IN ('ratings_moved','rating_conflicts_resolved')) AND EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid=to_regclass(current_schema() || '.catalog_operation_receipts') AND conname='catalog_operation_receipts_operation_check' AND pg_get_constraintdef(oid) LIKE '%mergeTeas%')`).Scan(&ready)

	return ready, wrapCatalogStorageError(err)
}

func validTeaMergePair(source, target uuid.UUID) bool {
	return source != uuid.Nil && target != uuid.Nil && source != target
}

func (d *db) PreviewTeaMerge(ctx context.Context, source, target uuid.UUID) (*common.TeaMergePreview, error) {
	if !validTeaMergePair(source, target) {
		return nil, common.ErrInvalidTeaMerge
	}

	tx, err := d.beginTeaMerge(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	preview, err := readTeaMergePreview(ctx, tx, source, target)
	if err != nil {
		return nil, err
	}

	return preview, wrapCatalogStorageError(tx.Commit())
}

func (d *db) MergeTeas(ctx context.Context, command common.TeaMergeCommand) (*common.TeaMergeResult, error) {
	if err := validateTeaMergeCommand(ctx, command); err != nil {
		return nil, err
	}

	tx, err := d.beginTeaMerge(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	hash, err := operationHash(command)
	if err != nil {
		return nil, err
	}

	replay, err := replayReceipt(ctx, tx, catalogMergeOperation, hash)
	if err != nil {
		return nil, err
	}

	if replay != nil {
		result, err := readTeaMergeResult(ctx, tx, replay, true)
		if err != nil {
			return nil, err
		}

		return result, wrapCatalogStorageError(tx.Commit())
	}

	result, err := applyTeaMerge(ctx, tx, command, hash)
	if err != nil {
		return nil, err
	}

	return result, wrapCatalogStorageError(tx.Commit())
}

func applyTeaMerge(ctx context.Context, tx *sql.Tx, command common.TeaMergeCommand, hash string) (*common.TeaMergeResult, error) {
	preview, err := readTeaMergePreview(ctx, tx, command.SourceTeaID, command.TargetTeaID)
	if err != nil {
		return nil, err
	}

	if err := validateTeaMergePreview(command, preview); err != nil {
		return nil, err
	}

	if err := transferTeaMergeDependencies(ctx, tx, preview); err != nil {
		return nil, err
	}

	if err := saveReceipt(ctx, tx, catalogMergeOperation, hash, command.TargetTeaID, command.SourceTeaID); err != nil {
		return nil, err
	}

	if err := saveTeaMergeDetails(ctx, tx, command, preview); err != nil {
		return nil, err
	}

	receipt, _, err := readReceipt(ctx, tx, common.CatalogueOperationFrom(ctx).Key)
	if err != nil {
		return nil, err
	}

	result, err := readTeaMergeResult(ctx, tx, receipt, false)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func validateTeaMergeCommand(ctx context.Context, command common.TeaMergeCommand) error {
	options := common.CatalogueOperationFrom(ctx)

	token, err := hex.DecodeString(command.PreviewToken)
	if !validTeaMergePair(command.SourceTeaID, command.TargetTeaID) || !options.HasKey || err != nil || len(token) != sha256.Size || hex.EncodeToString(token) != command.PreviewToken {
		return common.ErrInvalidTeaMerge
	}

	for _, revision := range []string{command.ExpectedSourceRevision, command.ExpectedTargetRevision} {
		if _, err := common.WithCatalogueOperation(ctx, &revision, nil, options.Actor); err != nil {
			return common.ErrInvalidTeaMerge
		}
	}

	return nil
}

func validateTeaMergePreview(command common.TeaMergeCommand, preview *common.TeaMergePreview) error {
	if preview.Source.Tea.Revision != command.ExpectedSourceRevision {
		return &common.CatalogueConflictError{CurrentRevision: preview.Source.Tea.Revision}
	}

	if preview.Target.Tea.Revision != command.ExpectedTargetRevision {
		return &common.CatalogueConflictError{CurrentRevision: preview.Target.Tea.Revision}
	}

	if preview.Token != command.PreviewToken {
		return common.ErrTeaMergePreviewConflict
	}

	if len(preview.Blockers) != 0 {
		return &common.TeaMergeBlockedError{Blockers: preview.Blockers}
	}

	return nil
}

// TeaMergeResultForReceipt recovers a committed merge without requiring surviving Teas.
func (d *db) TeaMergeResultForReceipt(ctx context.Context, command common.TeaMergeCommand) (*common.TeaMergeResult, error) {
	tx, err := d.pg.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, fmt.Errorf("begin Tea merge receipt read: %w", err)
	}
	defer tx.Rollback()

	if err := validateTeaMergeCommand(ctx, command); err != nil {
		return nil, err
	}

	hash, err := operationHash(command)
	if err != nil {
		return nil, err
	}

	receipt, err := replayReceipt(ctx, tx, catalogMergeOperation, hash)
	if err != nil {
		return nil, err
	}

	if receipt == nil {
		return nil, common.ErrTeaNotFound
	}

	result, err := readTeaMergeResult(ctx, tx, receipt, true)
	if err != nil {
		return nil, err
	}

	return result, wrapCatalogStorageError(tx.Commit())
}

func readTeaMergeResult(ctx context.Context, tx *sql.Tx, receipt *common.CatalogueReceipt, replayed bool) (*common.TeaMergeResult, error) {
	result := &common.TeaMergeResult{Receipt: receipt, Replayed: replayed}

	err := tx.QueryRowContext(ctx, `SELECT qr_count,collection_membership_count,tags_added,images_moved,consumptions_moved,ratings_moved,rating_conflicts_resolved FROM tea_merge_receipt_details WHERE operation_key=$1`, receipt.Key).
		Scan(&result.Counts.QRCount, &result.Counts.CollectionMembershipCount, &result.Counts.TagsAdded, &result.Counts.ImagesMoved, &result.Counts.ConsumptionsMoved, &result.Counts.RatingsMoved, &result.Counts.RatingConflictsResolved)
	if err != nil {
		return nil, fmt.Errorf("read Tea merge completion: %w", err)
	}

	target, err := scanCatalogueTea(tx.QueryRowContext(ctx, `SELECT `+teaColumns+` FROM teas WHERE id=$1`, receipt.TeaID))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if err == nil {
		result.Target = target

		snapshot, snapshotErr := readTeaMergeSnapshot(ctx, tx, target)
		if snapshotErr != nil {
			return nil, snapshotErr
		}

		result.TargetSnapshot = &snapshot
	}

	return result, nil
}

func saveTeaMergeDetails(ctx context.Context, tx *sql.Tx, command common.TeaMergeCommand, preview *common.TeaMergePreview) error {
	source, _ := json.Marshal(preview.Source.Tea.TeaData)
	target, _ := json.Marshal(preview.Target.Tea.TeaData)
	counts := preview.Counts
	_, err := tx.ExecContext(ctx, `INSERT INTO tea_merge_receipt_details(operation_key,source_revision,target_revision,preview_token,qr_count,collection_membership_count,tags_added,images_moved,consumptions_moved,source_metadata,target_metadata,ratings_moved,rating_conflicts_resolved) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11::jsonb,$12,$13)`, common.CatalogueOperationFrom(ctx).Key, command.ExpectedSourceRevision, command.ExpectedTargetRevision, command.PreviewToken, counts.QRCount, counts.CollectionMembershipCount, counts.TagsAdded, counts.ImagesMoved, counts.ConsumptionsMoved, string(source), string(target), counts.RatingsMoved, counts.RatingConflictsResolved)

	return wrapCatalogStorageError(err)
}

func transferTeaMergeDependencies(ctx context.Context, tx *sql.Tx, preview *common.TeaMergePreview) error {
	source, target := preview.Source.Tea.ID, preview.Target.Tea.ID

	statements := []string{
		`INSERT INTO tea_ratings(user_id,tea_id,rating,updated_at) SELECT user_id,$2,rating,updated_at FROM tea_ratings WHERE tea_id=$1 ON CONFLICT(user_id,tea_id) DO UPDATE SET rating=EXCLUDED.rating,updated_at=EXCLUDED.updated_at WHERE EXCLUDED.updated_at > tea_ratings.updated_at`,
		`UPDATE qr_records SET tea_id=$2 WHERE tea_id=$1`,
		`INSERT INTO tea_tags(tea_id,tag_id) SELECT $2,tag_id FROM tea_tags WHERE tea_id=$1 ON CONFLICT DO NOTHING`,
		`UPDATE tea_images SET is_cover=false WHERE tea_id=$1 AND is_cover AND EXISTS(SELECT 1 FROM tea_images WHERE tea_id=$2 AND is_cover)`,
		`UPDATE tea_images SET tea_id=$2 WHERE tea_id=$1`,
		`UPDATE consumptions SET tea_id=$2 WHERE tea_id=$1`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement, source, target); err != nil {
			return fmt.Errorf("transfer Tea merge dependencies: %w", err)
		}
	}
	// Imported galleries can lack a cover; select the oldest image only in that case.
	if preview.ResultingCoverImageID != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE tea_images SET is_cover=true WHERE tea_id=$1 AND id=$2 AND NOT EXISTS(SELECT 1 FROM tea_images WHERE tea_id=$1 AND is_cover)`, target, *preview.ResultingCoverImageID); err != nil {
			return fmt.Errorf("ensure merged Tea cover: %w", err)
		}
	}
	// Bump even when neither tags nor images changed. Triggers own the exact value.
	if _, err := tx.ExecContext(ctx, `UPDATE teas SET revision=revision WHERE id=$1`, target); err != nil {
		return fmt.Errorf("advance merged Tea revision: %w", err)
	}

	if _, err := tx.ExecContext(ctx, catalogDeleteTeaSQL, source); err != nil {
		return fmt.Errorf("remove merged Tea source: %w", err)
	}

	return nil
}
