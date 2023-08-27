package expiration

import (
	"context"
	"fmt"
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/common"
)

type Alerter interface {
	Start() error
	Stop() error
	Run(ctx context.Context) error
}

type sender interface {
	Send(ctx context.Context, userID uuid.UUID, title, body string) error
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
		cols, err := a.storage.Collections(ctx, user.ID)
		if err != nil {
			return err
		}

		for _, col := range cols {
			records, err := a.storage.CollectionRecords(ctx, col.ID)
			if err != nil {
				return err
			}

			for _, record := range records {
				if !record.ExpirationDate.After(time.Now()) {
					continue
				}

				body := fmt.Sprintf("tea %s from collection %s expired %s", record.Tea.Name, col.Name, record.ExpirationDate)

				if err = a.sender.Send(ctx, user.ID, "tea expired", body); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (a *alerter) loop() {
	ticker := time.NewTicker(time.Hour * 24)
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
