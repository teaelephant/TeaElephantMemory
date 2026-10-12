package graphql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/teaelephant/TeaElephantMemory/common"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

const cachedSelectionReadAttempts = 2

func (r *Resolver) refreshTeaOfDaySelection(ctx context.Context, cached *model.TeaOfTheDay) (*model.TeaOfTheDay, error) {
	// Retry once if a merge committed between reading the QR and its previous Tea.
	for range cachedSelectionReadAttempts {
		label, err := r.qrManager.Get(ctx, uuid.UUID(cached.Tea.ID))
		if err != nil {
			return nil, fmt.Errorf("refresh cached recommendation label: %w", err)
		}

		tea, err := r.teaData.Get(ctx, uuid.UUID(label.Tea))
		if errors.Is(err, common.ErrTeaNotFound) {
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("refresh cached recommendation Tea: %w", err)
		}

		return &model.TeaOfTheDay{Date: cached.Date, Tea: &model.QRRecord{ID: cached.Tea.ID,
			Tea: model.FromCommonTea(tea), BowlingTemp: label.BowlingTemp, ExpirationDate: label.ExpirationDate}}, nil
	}

	return nil, common.ErrTeaNotFound
}

func (r *Resolver) finalizeTeaOfDaySelection(ctx context.Context, userID uuid.UUID, selection *model.TeaOfTheDay) (*model.TeaOfTheDay, error) {
	refreshed, err := r.refreshTeaOfDaySelection(ctx, selection)
	if err != nil {
		return nil, err
	}

	if recorder, ok := r.consumption.(interface {
		RecordQR(ctx context.Context, userID uuid.UUID, qrID uuid.UUID, ts time.Time) (*common.CollectionRecord, error)
	}); ok {
		_, err = recorder.RecordQR(ctx, userID, uuid.UUID(refreshed.Tea.ID), refreshed.Date)
	} else {
		err = r.consumption.Record(ctx, userID, uuid.UUID(refreshed.Tea.Tea.ID), refreshed.Date)
	}

	if err != nil && r.log != nil {
		r.log.WithField(logKeyUser, userID.String()).WithField(logKeyErr, err).Debug("tea_of_day record consumption failed")
	}
	// Never retry an ambiguous history write; refresh only the same QR for return/cache.
	return r.refreshTeaOfDaySelection(ctx, refreshed)
}
