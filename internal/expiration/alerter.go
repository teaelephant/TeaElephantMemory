package expiration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/common"
)

//go:generate mockgen -source=alerter.go -destination=mocks/alerter.go -package=mocks storage,alerter,sender

type Alerter interface {
	Start() error
	Stop() error
	Run(ctx context.Context) error
}

type sender interface {
	Send(ctx context.Context, userID, itemID uuid.UUID, title, body string) error
}

type storage interface {
	GetUsers(ctx context.Context) ([]common.User, error)
	Collections(ctx context.Context, userID uuid.UUID) ([]*common.Collection, error)
	CollectionRecords(ctx context.Context, id uuid.UUID) ([]*common.CollectionRecord, error)
}

type alerter struct {
	sender
	storage

	startSync *sync.Once

	log  *logrus.Entry
	stop chan struct{}
}

func (a *alerter) Start() error {
	a.startSync.Do(func() {
		go a.loop()
	})

	return nil
}

func (a *alerter) Stop() error {
	a.stop <- struct{}{}
	return nil
}

func (a *alerter) Run(ctx context.Context) error {
	users, err := a.storage.GetUsers(ctx)
	if err != nil {
		return err
	}
	for _, user := range users {
		if err := a.processUserCollections(ctx, user.ID); err != nil {
			return err
		}
	}
	return nil
}

func (a *alerter) processUserCollections(ctx context.Context, userID uuid.UUID) error {
	collections, err := a.storage.Collections(ctx, userID)
	if err != nil {
		return err
	}

	for _, col := range collections {
		if err := a.processCollectionRecords(ctx, userID, col); err != nil {
			return err
		}
	}
	return nil
}

func (a *alerter) processCollectionRecords(ctx context.Context, userID uuid.UUID, col *common.Collection) error {
	records, err := a.storage.CollectionRecords(ctx, col.ID)
	if err != nil {
		return err
	}

	for _, record := range records {
		if err := a.processRecord(ctx, userID, col, record); err != nil {
			return err
		}
	}
	return nil
}

func (a *alerter) processRecord(ctx context.Context, userID uuid.UUID, col *common.Collection, record *common.CollectionRecord) error {
	a.log.WithField("user", userID).
		WithField("collection", col.Name).
		WithField("tea", record.Tea.Name).Debug("expiration date checked")

	if !record.ExpirationDate.Before(time.Now()) {
		return nil
	}

	body := fmt.Sprintf("tea %s from collection %s expired %s", record.Tea.Name, col.Name, record.ExpirationDate)
	return a.sender.Send(ctx, userID, record.ID, "tea expired", body)
}

func (a *alerter) loop() {
	ticker := time.NewTicker(time.Hour * 24 * 7)
loop:
	for {
		select {
		case <-a.stop:
			break loop
		case <-ticker.C:
			if err := a.Run(context.Background()); err != nil {
				a.log.WithError(err).Error("alerter run")
			}
		}
	}
	close(a.stop)
}

func NewAlerter(sender sender, storage storage, log *logrus.Entry) Alerter {
	return &alerter{sender: sender, storage: storage, log: log, startSync: new(sync.Once), stop: make(chan struct{})}
}
