package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/teaelephant/TeaElephantMemory/common"
)

const teaRatingEligibility = `EXISTS(SELECT 1 FROM collections c JOIN collection_qr_items i ON i.collection_id=c.id JOIN qr_records q ON q.id=i.qr_id WHERE c.user_id=$2 AND q.tea_id=$1)`

// TeaRating reads aggregates and viewer-specific fields in one statement/snapshot.
func (d *db) TeaRating(ctx context.Context, teaID, userID uuid.UUID) (*common.TeaRating, error) {
	return readTeaRating(ctx, d.pg, teaID, userID)
}

type teaRatingReader interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func readTeaRating(ctx context.Context, reader teaRatingReader, teaID, userID uuid.UUID) (*common.TeaRating, error) {
	value := &common.TeaRating{TeaID: teaID}

	err := reader.QueryRowContext(ctx, `SELECT (SELECT avg(rating)::float8 FROM tea_ratings WHERE tea_id=$1),(SELECT count(*) FROM tea_ratings WHERE tea_id=$1),(SELECT rating FROM tea_ratings WHERE tea_id=$1 AND user_id=$2),`+teaRatingEligibility+` FROM teas WHERE id=$1`, teaID, userID).Scan(&value.Average, &value.Count, &value.MyRating, &value.CanRate)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, common.ErrTeaNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("read Tea rating: %w", err)
	}

	return value, nil
}

// SetTeaRating serializes eligibility, persistence and merge preview invalidation.
func (d *db) SetTeaRating(ctx context.Context, teaID, userID uuid.UUID, rating int) (*common.TeaRating, error) {
	if rating < 1 || rating > 5 {
		return nil, common.ErrInvalidTeaRating
	}

	return d.changeTeaRating(ctx, teaID, userID, &rating)
}

// ClearTeaRating permits the user to remove a retained vote without membership.
func (d *db) ClearTeaRating(ctx context.Context, teaID, userID uuid.UUID) (*common.TeaRating, error) {
	return d.changeTeaRating(ctx, teaID, userID, nil)
}

func (d *db) changeTeaRating(ctx context.Context, teaID, userID uuid.UUID, rating *int) (*common.TeaRating, error) {
	if userID == uuid.Nil {
		return nil, common.ErrUnauthorized
	}
	// The same catalog advisory and dependency-table lock order as a merge.
	// Legacy membership/QR removals finish before eligibility is read; they
	// cannot remove the qualifying membership before this transaction commits.
	tx, err := d.beginTeaMerge(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	current, err := readTeaRating(ctx, tx, teaID, userID)
	if err != nil {
		return nil, err
	}

	if rating != nil {
		if !current.CanRate {
			return nil, common.ErrTeaRatingIneligible
		}

		_, err = tx.ExecContext(ctx, `INSERT INTO tea_ratings(user_id,tea_id,rating) VALUES($1,$2,$3) ON CONFLICT(user_id,tea_id) DO UPDATE SET rating=EXCLUDED.rating,updated_at=clock_timestamp() WHERE tea_ratings.rating IS DISTINCT FROM EXCLUDED.rating`, userID, teaID, *rating)
	} else {
		_, err = tx.ExecContext(ctx, `DELETE FROM tea_ratings WHERE user_id=$1 AND tea_id=$2`, userID, teaID)
	}

	if err != nil {
		return nil, fmt.Errorf("change Tea rating: %w", err)
	}

	result, err := readTeaRating(ctx, tx, teaID, userID)
	if err != nil {
		return nil, err
	}

	return result, wrapCatalogStorageError(tx.Commit())
}
