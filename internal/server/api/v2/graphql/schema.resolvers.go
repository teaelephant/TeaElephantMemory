package graphql

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"

	"github.com/teaelephant/TeaElephantMemory/internal/server/api/v2/graphql/generated"
	model "github.com/teaelephant/TeaElephantMemory/internal/server/api/v2/models"
)

func (r *mutationResolver) NewTea(ctx context.Context, tea model.TeaData) (*model.Tea, error) {
	res, err := r.teaData.Create(tea.ToCommonTeaData())
	if err != nil {
		return nil, err
	}
	return model.FromCommonTea(res), nil
}

func (r *mutationResolver) UpdateTea(ctx context.Context, id string, tea model.TeaData) (*model.Tea, error) {
	res, err := r.teaData.Update(id, tea.ToCommonTeaData())
	if err != nil {
		return nil, err
	}
	return model.FromCommonTea(res), nil
}

func (r *mutationResolver) DeleteTea(ctx context.Context, id string) (string, error) {
	if err := r.teaData.Delete(id); err != nil {
		return "", err
	}
	return id, nil
}

func (r *mutationResolver) WriteToQr(ctx context.Context, id string, data model.QRRecordData) (*model.QRRecord, error) {
	if err := r.qrManager.Set(id, &data); err != nil {
		return nil, err
	}
	tea, err := r.teaData.Get(data.Tea)
	if err != nil {
		return nil, err
	}
	return &model.QRRecord{
		ID: id,
		Tea: &model.Tea{
			ID:          tea.ID,
			Name:        tea.Name,
			Type:        model.Type(tea.Type),
			Description: tea.Description,
		},
		BowlingTemp:    data.BowlingTemp,
		ExpirationDate: data.ExpirationDate,
	}, nil
}

func (r *mutationResolver) CreateTag(ctx context.Context, name string, color string) (*model.Tag, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) UpdateTag(ctx context.Context, id string, name string, color string) (*model.Tag, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) DeleteTag(ctx context.Context, id string) (*string, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) GetTeas(ctx context.Context, prefix *string) ([]*model.Tea, error) {
	res, err := r.teaData.List(prefix)
	if err != nil {
		return nil, err
	}
	data := make([]*model.Tea, len(res))
	for i, el := range res {
		data[i] = model.FromCommonTea(&el)
	}
	return data, nil
}

func (r *queryResolver) GetTea(ctx context.Context, id string) (*model.Tea, error) {
	res, err := r.teaData.Get(id)
	if err != nil {
		return nil, err
	}
	return model.FromCommonTea(res), nil
}

func (r *queryResolver) GetQrRecord(ctx context.Context, id string) (*model.QRRecord, error) {
	data, err := r.qrManager.Get(id)
	if err != nil {
		return nil, err
	}
	tea, err := r.teaData.Get(data.Tea)
	if err != nil {
		return nil, err
	}
	return &model.QRRecord{
		ID: id,
		Tea: &model.Tea{
			ID:          tea.ID,
			Name:        tea.Name,
			Type:        model.Type(tea.Type),
			Description: tea.Description,
		},
		BowlingTemp:    data.BowlingTemp,
		ExpirationDate: data.ExpirationDate,
	}, nil
}

func (r *queryResolver) GetTag(ctx context.Context, id string) (*model.Tag, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) GetTags(ctx context.Context, name *string) ([]*model.Tag, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *subscriptionResolver) OnCreateTea(ctx context.Context) (<-chan *model.Tea, error) {
	return r.teaData.SubscribeOnCreate()
}

func (r *subscriptionResolver) OnUpdateTea(ctx context.Context) (<-chan *model.Tea, error) {
	return r.teaData.SubscribeOnUpdate()
}

func (r *subscriptionResolver) OnDeleteTea(ctx context.Context) (<-chan string, error) {
	return r.teaData.SubscribeOnDelete()
}

func (r *subscriptionResolver) OnCreateTag(ctx context.Context) (<-chan *model.Tag, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *subscriptionResolver) OnUpdateTag(ctx context.Context) (<-chan *model.Tag, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *subscriptionResolver) OnDeleteTag(ctx context.Context) (<-chan string, error) {
	panic(fmt.Errorf("not implemented"))
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

// Subscription returns generated.SubscriptionResolver implementation.
func (r *Resolver) Subscription() generated.SubscriptionResolver { return &subscriptionResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }
