// Package pg contains the Postgres-backed storage adapter used by managers.
//
//nolint:wsl_v5 // legacy file style; whitespace rules disabled to minimize churn
package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/pkg/pgstore"
)

// db is a Postgres-backed storage that implements the method sets required by
// managers (tea, tag, qr, collection, notification) and auth. It delegates SQL
// execution to sqlc-generated helpers in pkg/pgstore.
type db struct {
	pg      *sql.DB
	log     *logrus.Entry
	queries *pgstore.Queries
}

// NewDB creates a Postgres-backed adapter instance.
// revive:disable:unexported-return // internal package consumers depend on concrete *db; exporting the type is out-of-scope.
func NewDB(pg *sql.DB, log *logrus.Entry) *db {
	return &db{pg: pg, log: log, queries: pgstore.New(pg)}
}

// ===== Users =====

func (d *db) GetOrCreateUser(ctx context.Context, unique string) (uuid.UUID, error) {
	user, err := d.queries.UpsertUser(ctx, pgstore.UpsertUserParams{ID: uuid.New(), AppleID: unique})
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert user: %w", err)
	}
	return user.ID, nil
}

func (d *db) GetUsers(ctx context.Context) ([]common.User, error) {
	users, err := d.queries.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	res := make([]common.User, 0, len(users))
	for _, u := range users {
		res = append(res, common.User{ID: u.ID, AppleID: u.AppleID})
	}
	return res, nil
}

// ===== QR =====

func (d *db) WriteQR(ctx context.Context, id uuid.UUID, data *common.QR) error {
	// Clamp to int32 range to avoid overflow (gosec G115)
	bt := data.BowlingTemp
	if bt > math.MaxInt32 {
		bt = math.MaxInt32
	} else if bt < math.MinInt32 {
		bt = math.MinInt32
	}
	if err := d.queries.UpsertQR(ctx, pgstore.QRRecord{
		ID:             id,
		TeaID:          data.Tea,
		BoilingTemp:    int32(bt), //nolint:gosec // domain: boiling temp is bounded (0..100C), clamped above
		ExpirationDate: data.ExpirationDate.UTC(),
	}); err != nil {
		return fmt.Errorf("upsert qr: %w", err)
	}
	return nil
}

func (d *db) ReadQR(ctx context.Context, id uuid.UUID) (*common.QR, error) {
	qr, err := d.queries.GetQR(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, common.ErrQRRecordNotExist
		}
		return nil, fmt.Errorf("get qr: %w", err)
	}
	return &common.QR{
		Tea:            qr.TeaID,
		BowlingTemp:    int(qr.BoilingTemp),
		ExpirationDate: qr.ExpirationDate,
	}, nil
}

// ===== Collections =====

func (d *db) CreateCollection(ctx context.Context, userID uuid.UUID, name string) (uuid.UUID, error) {
	col, err := d.queries.InsertCollection(ctx, pgstore.InsertCollectionParams{ID: uuid.New(), UserID: userID, Name: name})
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert collection: %w", err)
	}
	return col.ID, nil
}

func (d *db) AddTeaToCollection(ctx context.Context, id uuid.UUID, teas []uuid.UUID) error {
	if len(teas) == 0 {
		// Nothing to do
		d.log.WithField("collection_id", id).Debug("AddTeaToCollection called with empty teas slice; skipping")
		return nil
	}

	entry := d.log.WithFields(logrus.Fields{
		"collection_id": id,
		"count":         len(teas),
	})
	entry.Debug("adding teas to collection")

	if err := d.queries.InsertCollectionItems(ctx, id, teas); err != nil {
		entry.WithError(err).Error("failed to add teas to collection")
		return fmt.Errorf("add qr to collection (batch): %w", err)
	}

	entry.Info("teas added to collection")
	return nil
}

func (d *db) DeleteTeaFromCollection(ctx context.Context, id uuid.UUID, teas []uuid.UUID) error {
	for _, qrID := range teas {
		if err := d.queries.DeleteCollectionItem(ctx, id, qrID); err != nil {
			return fmt.Errorf("delete qr from collection: %w", err)
		}
	}
	return nil
}

func (d *db) DeleteCollection(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	if err := d.queries.DeleteCollection(ctx, id, userID); err != nil {
		return fmt.Errorf("delete collection: %w", err)
	}
	return nil
}

func (d *db) Collections(ctx context.Context, userID uuid.UUID) ([]*common.Collection, error) {
	cols, err := d.queries.ListCollections(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	res := make([]*common.Collection, 0, len(cols))
	for _, col := range cols {
		res = append(res, &common.Collection{ID: col.ID, Name: col.Name})
	}
	return res, nil
}

func (d *db) Collection(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*common.Collection, error) {
	col, err := d.queries.GetCollection(ctx, id, userID)
	if err != nil {
		return nil, fmt.Errorf("get collection: %w", err)
	}
	return &common.Collection{ID: col.ID, Name: col.Name}, nil
}

func (d *db) CollectionRecords(ctx context.Context, id uuid.UUID) ([]*common.CollectionRecord, error) {
	rows, err := d.queries.ListCollectionRecords(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list collection records: %w", err)
	}
	res := make([]*common.CollectionRecord, 0, len(rows))
	for _, row := range rows {
		rec := &common.CollectionRecord{
			ID: row.QRID,
			Tea: &common.Tea{
				ID: row.TeaID,
				TeaData: &common.TeaData{
					Name:        row.Name,
					Type:        common.StringToBeverageType(row.Type),
					Description: nullableString(row.Description),
				},
			},
			BowlingTemp:    int(row.BoilingTemp),
			ExpirationDate: row.ExpirationDate,
		}
		res = append(res, rec)
	}
	return res, nil
}

// ===== Notifications & Devices =====

func (d *db) AddDeviceForUser(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) error {
	if err := d.queries.InsertDevice(ctx, deviceID, userID, deviceID.String()); err != nil {
		return fmt.Errorf("insert device: %w", err)
	}
	return nil
}

func (d *db) CreateOrUpdateDeviceToken(ctx context.Context, userID, deviceID uuid.UUID, deviceToken string) error {
	result, err := d.pg.ExecContext(ctx, `UPDATE devices SET token = $2 WHERE id = $1 AND user_id = $3`, deviceID, deviceToken, userID)
	if err != nil {
		return fmt.Errorf("update device token: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated device tokens: %w", err)
	}
	if rows == 0 {
		return common.ErrDeviceNotFound
	}
	return nil
}

func (d *db) Notifications(ctx context.Context, userID uuid.UUID) ([]common.Notification, error) {
	notifs, err := d.queries.ListNotifications(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	res := make([]common.Notification, 0, len(notifs))
	for _, n := range notifs {
		res = append(res, common.Notification{UserID: userID, Type: common.NotificationType(n.Type)})
	}
	return res, nil
}

func (d *db) MapUserIDToDeviceID(ctx context.Context, userID uuid.UUID) ([]string, error) {
	tokens, err := d.queries.ListDeviceTokens(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list device tokens: %w", err)
	}
	return tokens, nil
}

// ===== Helpers =====

func nullableString(src sql.NullString) string {
	if src.Valid {
		return src.String
	}
	return ""
}
