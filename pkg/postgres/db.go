package postgres

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/common"
)

// Interface definitions for embedded interfaces
type qr interface {
	WriteQR(ctx context.Context, id uuid.UUID, data *common.QR) (err error)
	ReadQR(ctx context.Context, id uuid.UUID) (record *common.QR, err error)
}

type record interface {
	WriteRecord(ctx context.Context, rec *common.TeaData) (record *common.Tea, err error)
	ReadRecord(ctx context.Context, id uuid.UUID) (record *common.Tea, err error)
	ReadAllRecords(ctx context.Context, search string) ([]common.Tea, error)
	Update(ctx context.Context, id uuid.UUID, rec *common.TeaData) (record *common.Tea, err error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type version interface {
	GetVersion(ctx context.Context) (uint32, error)
	WriteVersion(ctx context.Context, version uint32) error
}

type tag interface {
	CreateTagCategory(ctx context.Context, name string) (category *common.TagCategory, err error)
	UpdateTagCategory(ctx context.Context, id uuid.UUID, name string) error
	DeleteTagCategory(ctx context.Context, id uuid.UUID) (removedTags []uuid.UUID, err error)
	GetTagCategory(ctx context.Context, id uuid.UUID) (category *common.TagCategory, err error)
	ListTagCategories(ctx context.Context, search *string) (list []common.TagCategory, err error)
	CreateTag(ctx context.Context, name, color string, categoryID uuid.UUID) (*common.Tag, error)
	UpdateTag(ctx context.Context, id uuid.UUID, name, color string) (*common.Tag, error)
	ChangeTagCategory(ctx context.Context, id, categoryID uuid.UUID) (*common.Tag, error)
	DeleteTag(ctx context.Context, id uuid.UUID) error
	GetTag(ctx context.Context, id uuid.UUID) (*common.Tag, error)
	ListTags(ctx context.Context, name *string, categoryID *uuid.UUID) (list []common.Tag, err error)
	AddTagToTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error
	DeleteTagFromTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error
	ListByTea(ctx context.Context, id uuid.UUID) ([]common.Tag, error)
}

type collection interface {
	CreateCollection(ctx context.Context, userID uuid.UUID, name string) (uuid.UUID, error)
	AddTeaToCollection(ctx context.Context, id uuid.UUID, teas []uuid.UUID) error
	DeleteTeaFromCollection(ctx context.Context, id uuid.UUID, teas []uuid.UUID) error
	DeleteCollection(ctx context.Context, id, userID uuid.UUID) error
	Collections(ctx context.Context, userID uuid.UUID) ([]*common.Collection, error)
	Collection(ctx context.Context, id, userID uuid.UUID) (*common.Collection, error)
	CollectionRecords(ctx context.Context, id uuid.UUID) ([]*common.CollectionRecord, error)
}

type notification interface {
	CreateOrUpdateDeviceToken(ctx context.Context, deviceID uuid.UUID, deviceToken string) error
	Notifications(ctx context.Context, userID uuid.UUID) ([]common.Notification, error)
	AddDeviceForUser(ctx context.Context, userID, deviceID uuid.UUID) error
	MapUserIdToDeviceID(ctx context.Context, userID uuid.UUID) ([]string, error)
}

// DB interface mirrors the FDB interface to ensure compatibility
type DB interface {
	qr
	record
	version
	tag
	collection
	notification

	GetOrCreateUser(ctx context.Context, unique string) (uuid.UUID, error)
	GetUsers(ctx context.Context) ([]common.User, error)
}

type pgDB struct {
	dbConn  *sql.DB
	log *logrus.Entry
}

func (d *pgDB) GetUsers(ctx context.Context) ([]common.User, error) {
	rows, err := d.dbConn.QueryContext(ctx, "SELECT id, apple_id FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]common.User, 0)
	for rows.Next() {
		var user common.User
		var id []byte
		if err := rows.Scan(&id, &user.AppleID); err != nil {
			return nil, err
		}
		if err := user.ID.UnmarshalBinary(id); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (d *pgDB) GetOrCreateUser(ctx context.Context, unique string) (uuid.UUID, error) {
	var userID uuid.UUID
	var id []byte

	// Try to get existing user
	err := d.dbConn.QueryRowContext(ctx, "SELECT id FROM users WHERE apple_id = $1", unique).Scan(&id)
	if err == nil {
		if err := userID.UnmarshalBinary(id); err != nil {
			return uuid.Nil, err
		}
		return userID, nil
	} else if err != sql.ErrNoRows {
		return uuid.Nil, err
	}

	// Create new user
	userID = uuid.New()
	id, err = userID.MarshalBinary()
	if err != nil {
		return uuid.Nil, err
	}

	_, err = d.dbConn.ExecContext(ctx, "INSERT INTO users (id, apple_id) VALUES ($1, $2)", id, unique)
	if err != nil {
		return uuid.Nil, err
	}

	return userID, nil
}

// NewDB creates a new PostgreSQL DB instance
func NewDB(db *sql.DB, log *logrus.Entry) DB {
	return &pgDB{
		dbConn:  db,
		log: log,
	}
}
