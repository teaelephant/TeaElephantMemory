// Package pg contains the Postgres-backed storage adapter used by managers.
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

// ===== Teas (records) =====

func (d *db) WriteRecord(ctx context.Context, rec *common.TeaData) (*common.Tea, error) {
	tea, err := d.queries.InsertTea(ctx, pgstore.InsertTeaParams{
		ID:          uuid.New(),
		Name:        rec.Name,
		Type:        rec.Type.String(),
		Description: sql.NullString{String: rec.Description, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("insert tea: %w", err)
	}
	return &common.Tea{ID: tea.ID, TeaData: &common.TeaData{
		Name:        tea.Name,
		Type:        common.StringToBeverageType(tea.Type),
		Description: nullableString(tea.Description),
	}}, nil
}

func (d *db) ReadRecord(ctx context.Context, id uuid.UUID) (*common.Tea, error) {
	tea, err := d.queries.GetTea(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("tea not found: %w", err)
		}
		return nil, fmt.Errorf("get tea: %w", err)
	}
	return &common.Tea{ID: tea.ID, TeaData: &common.TeaData{
		Name:        tea.Name,
		Type:        common.StringToBeverageType(tea.Type),
		Description: nullableString(tea.Description),
	}}, nil
}

func (d *db) ReadAllRecords(ctx context.Context, search string) ([]common.Tea, error) {
	var (
		teas []pgstore.Tea
		err  error
	)
	if search == "" {
		teas, err = d.queries.ListTeas(ctx)
	} else {
		teas, err = d.queries.SearchTeasByPrefix(ctx, search, int32(1<<31-1))
	}
	if err != nil {
		return nil, fmt.Errorf("list teas: %w", err)
	}
	res := make([]common.Tea, 0, len(teas))
	for _, t := range teas {
		td := common.TeaData{
			Name:        t.Name,
			Type:        common.StringToBeverageType(t.Type),
			Description: nullableString(t.Description),
		}
		res = append(res, common.Tea{ID: t.ID, TeaData: &td})
	}
	return res, nil
}

func (d *db) Update(ctx context.Context, id uuid.UUID, rec *common.TeaData) (*common.Tea, error) {
	tea, err := d.queries.UpdateTea(ctx, pgstore.UpdateTeaParams{
		ID:          id,
		Name:        rec.Name,
		Type:        rec.Type.String(),
		Description: sql.NullString{String: rec.Description, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("update tea: %w", err)
	}
	return &common.Tea{ID: tea.ID, TeaData: &common.TeaData{
		Name:        tea.Name,
		Type:        common.StringToBeverageType(tea.Type),
		Description: nullableString(tea.Description),
	}}, nil
}

func (d *db) Delete(ctx context.Context, id uuid.UUID) error {
	if err := d.queries.DeleteTea(ctx, id); err != nil {
		return fmt.Errorf("delete tea: %w", err)
	}
	return nil
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

// ===== Tags & Categories =====

func (d *db) CreateTagCategory(ctx context.Context, name string) (*common.TagCategory, error) {
	cat, err := d.queries.InsertTagCategory(ctx, pgstore.InsertTagCategoryParams{ID: uuid.New(), Name: name})
	if err != nil {
		return nil, fmt.Errorf("insert tag category: %w", err)
	}
	return &common.TagCategory{ID: cat.ID, Name: cat.Name}, nil
}

func (d *db) UpdateTagCategory(ctx context.Context, id uuid.UUID, name string) error {
	if _, err := d.queries.UpdateTagCategory(ctx, pgstore.UpdateTagCategoryParams{ID: id, Name: name}); err != nil {
		return fmt.Errorf("update tag category: %w", err)
	}
	return nil
}

func (d *db) DeleteTagCategory(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	rows, err := d.queries.ListTagsByCategory(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list tags by category: %w", err)
	}
	removed := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		removed = append(removed, row.ID)
	}
	if err := d.queries.DeleteTagsByCategory(ctx, id); err != nil {
		return nil, fmt.Errorf("delete tags by category: %w", err)
	}
	if err := d.queries.DeleteTagCategory(ctx, id); err != nil {
		return nil, fmt.Errorf("delete category: %w", err)
	}
	return removed, nil
}

func (d *db) GetTagCategory(ctx context.Context, id uuid.UUID) (*common.TagCategory, error) {
	cat, err := d.queries.GetTagCategory(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("tag category not found: %w", err)
		}
		return nil, fmt.Errorf("get tag category: %w", err)
	}
	return &common.TagCategory{ID: cat.ID, Name: cat.Name}, nil
}

func (d *db) ListTagCategories(ctx context.Context, search *string) ([]common.TagCategory, error) {
	var (
		cats []pgstore.TagCategory
		err  error
	)
	if search == nil || *search == "" {
		cats, err = d.queries.ListTagCategories(ctx)
	} else {
		cats, err = d.queries.SearchTagCategories(ctx, *search)
	}
	if err != nil {
		return nil, fmt.Errorf("list tag categories: %w", err)
	}
	res := make([]common.TagCategory, 0, len(cats))
	for _, cat := range cats {
		res = append(res, common.TagCategory{ID: cat.ID, Name: cat.Name})
	}
	return res, nil
}

func (d *db) CreateTag(ctx context.Context, name, color string, categoryID uuid.UUID) (*common.Tag, error) {
	tag, err := d.queries.InsertTag(ctx, pgstore.InsertTagParams{
		ID:         uuid.New(),
		Name:       name,
		Color:      color,
		CategoryID: categoryID,
	})
	if err != nil {
		return nil, fmt.Errorf("insert tag: %w", err)
	}
	return &common.Tag{ID: tag.ID, TagData: &common.TagData{Name: tag.Name, Color: tag.Color, CategoryID: tag.CategoryID}}, nil
}

func (d *db) UpdateTag(ctx context.Context, id uuid.UUID, name, color string) (*common.Tag, error) {
	tag, err := d.queries.UpdateTag(ctx, pgstore.UpdateTagParams{ID: id, Name: name, Color: color})
	if err != nil {
		return nil, fmt.Errorf("update tag: %w", err)
	}
	return &common.Tag{ID: tag.ID, TagData: &common.TagData{Name: tag.Name, Color: tag.Color, CategoryID: tag.CategoryID}}, nil
}

func (d *db) ChangeTagCategory(ctx context.Context, id, categoryID uuid.UUID) (*common.Tag, error) {
	tag, err := d.queries.ChangeTagCategory(ctx, pgstore.ChangeTagCategoryParams{ID: id, CategoryID: categoryID})
	if err != nil {
		return nil, fmt.Errorf("change tag category: %w", err)
	}
	return &common.Tag{ID: tag.ID, TagData: &common.TagData{Name: tag.Name, Color: tag.Color, CategoryID: tag.CategoryID}}, nil
}

func (d *db) DeleteTag(ctx context.Context, id uuid.UUID) error {
	if err := d.queries.DeleteTag(ctx, id); err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	return nil
}

func (d *db) GetTag(ctx context.Context, id uuid.UUID) (*common.Tag, error) {
	tag, err := d.queries.GetTag(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get tag: %w", err)
	}
	return &common.Tag{ID: tag.ID, TagData: &common.TagData{Name: tag.Name, Color: tag.Color, CategoryID: tag.CategoryID}}, nil
}

func (d *db) ListTags(ctx context.Context, name *string, categoryID *uuid.UUID) ([]common.Tag, error) {
	var (
		tags []pgstore.Tag
		err  error
	)
	switch {
	case name == nil || *name == "":
		if categoryID == nil {
			tags, err = d.queries.ListTags(ctx)
		} else {
			tags, err = d.queries.ListTagsByCategoryFilter(ctx, *categoryID)
		}
	default:
		if categoryID == nil {
			tags, err = d.queries.ListTagsByName(ctx, *name)
		} else {
			tags, err = d.queries.ListTagsByNameCategory(ctx, *name, *categoryID)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	res := make([]common.Tag, 0, len(tags))
	for _, tag := range tags {
		res = append(res, common.Tag{ID: tag.ID, TagData: &common.TagData{Name: tag.Name, Color: tag.Color, CategoryID: tag.CategoryID}})
	}
	return res, nil
}

func (d *db) AddTagToTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error {
	if err := d.queries.AddTagToTea(ctx, tea, tag); err != nil {
		return fmt.Errorf("add tag to tea: %w", err)
	}
	return nil
}

func (d *db) DeleteTagFromTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error {
	if err := d.queries.DeleteTagFromTea(ctx, tea, tag); err != nil {
		return fmt.Errorf("delete tag from tea: %w", err)
	}
	return nil
}

func (d *db) ListByTea(ctx context.Context, id uuid.UUID) ([]common.Tag, error) {
	tags, err := d.queries.ListTagsByTea(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list tags by tea: %w", err)
	}
	res := make([]common.Tag, 0, len(tags))
	for _, tag := range tags {
		res = append(res, common.Tag{ID: tag.ID, TagData: &common.TagData{Name: tag.Name, Color: tag.Color, CategoryID: tag.CategoryID}})
	}
	return res, nil
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

func (d *db) CreateOrUpdateDeviceToken(ctx context.Context, deviceID uuid.UUID, deviceToken string) error {
	affected, err := d.queries.UpdateDeviceToken(ctx, deviceID, deviceToken)
	if err != nil {
		return fmt.Errorf("update device token: %w", err)
	}
	if affected == 0 {
		d.log.WithField("device_id", deviceID).Warn("device token update skipped: no matching device row")
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

func (d *db) MapUserIdToDeviceID(ctx context.Context, userID uuid.UUID) ([]string, error) {
	tokens, err := d.queries.ListDeviceTokens(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list device tokens: %w", err)
	}
	return tokens, nil
}

// ===== Version =====

func (d *db) GetVersion(_ context.Context) (uint32, error) {
	return 0, nil
}

func (d *db) WriteVersion(_ context.Context, _ uint32) error {
	return nil
}

// ===== Consumption Store =====

func nullableString(src sql.NullString) string {
	if src.Valid {
		return src.String
	}
	return ""
}
