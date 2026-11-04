package graphql

import (
	"context"

	"github.com/google/uuid"
	"github.com/teaelephant/TeaElephantMemory/common"
	authPkg "github.com/teaelephant/TeaElephantMemory/internal/auth"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

type teaRatingStore interface {
	TeaRating(ctx context.Context, teaID, userID uuid.UUID) (*common.TeaRating, error)
	SetTeaRating(ctx context.Context, teaID, userID uuid.UUID, rating int) (*common.TeaRating, error)
	ClearTeaRating(ctx context.Context, teaID, userID uuid.UUID) (*common.TeaRating, error)
}

func ratingModel(value *common.TeaRating) *model.TeaRating {
	return &model.TeaRating{TeaID: gqlCommon.ID(value.TeaID), Average: value.Average, Count: value.Count, MyRating: value.MyRating, CanRate: value.CanRate}
}

func (r *Resolver) readRating(ctx context.Context, teaID gqlCommon.ID) (*model.TeaRating, error) {
	if r.ratings == nil {
		return nil, castGQLError(ctx, common.ErrTeaRatingsUnavailable)
	}

	userID := uuid.Nil
	if user, err := authPkg.GetUser(ctx); err == nil {
		userID = user.ID
	}

	value, err := r.ratings.TeaRating(ctx, uuid.UUID(teaID), userID)
	if err != nil {
		return nil, castGQLError(ctx, err)
	}

	return ratingModel(value), nil
}

func (r *Resolver) changeRating(ctx context.Context, teaID gqlCommon.ID, rating *int) (*model.TeaRating, error) {
	user, err := authPkg.GetUser(ctx)
	if err != nil {
		return nil, castGQLError(ctx, common.ErrUnauthorized)
	}

	if r.ratings == nil {
		return nil, castGQLError(ctx, common.ErrTeaRatingsUnavailable)
	}

	var value *common.TeaRating
	if rating == nil {
		value, err = r.ratings.ClearTeaRating(ctx, uuid.UUID(teaID), user.ID)
	} else {
		value, err = r.ratings.SetTeaRating(ctx, uuid.UUID(teaID), user.ID, *rating)
	}

	if err != nil {
		return nil, castGQLError(ctx, err)
	}

	return ratingModel(value), nil
}
