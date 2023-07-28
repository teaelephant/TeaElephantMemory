package graphql

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
// Code generated by github.com/99designs/gqlgen version v0.17.35

import (
	"context"
	"fmt"

	"github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/graphql/generated"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

// NewTea is the resolver for the newTea field.
func (r *mutationResolver) NewTea(ctx context.Context, tea model.TeaData) (*model.Tea, error) {
	res, err := r.teaData.Create(ctx, tea.ToCommonTeaData())
	if err != nil {
		return nil, err
	}
	return model.FromCommonTea(res), nil
}

// UpdateTea is the resolver for the updateTea field.
func (r *mutationResolver) UpdateTea(ctx context.Context, id common.ID, tea model.TeaData) (*model.Tea, error) {
	res, err := r.teaData.Update(ctx, uuid.UUID(id), tea.ToCommonTeaData())
	if err != nil {
		return nil, err
	}
	return model.FromCommonTea(res), nil
}

// AddTagToTea is the resolver for the addTagToTea field.
func (r *mutationResolver) AddTagToTea(ctx context.Context, teaID common.ID, tagID common.ID) (*model.Tea, error) {
	if err := r.tagManager.AddTagToTea(ctx, uuid.UUID(teaID), uuid.UUID(tagID)); err != nil {
		return nil, err
	}
	t, err := r.teaData.Get(ctx, uuid.UUID(teaID))
	if err != nil {
		return nil, err
	}

	return model.FromCommonTea(t), nil
}

// DeleteTagFromTea is the resolver for the deleteTagFromTea field.
func (r *mutationResolver) DeleteTagFromTea(ctx context.Context, teaID common.ID, tagID common.ID) (*model.Tea, error) {
	if err := r.tagManager.DeleteTagFromTea(ctx, uuid.UUID(teaID), uuid.UUID(tagID)); err != nil {
		return nil, err
	}
	t, err := r.teaData.Get(ctx, uuid.UUID(teaID))
	if err != nil {
		return nil, err
	}
	return model.FromCommonTea(t), nil
}

// DeleteTea is the resolver for the deleteTea field.
func (r *mutationResolver) DeleteTea(ctx context.Context, id common.ID) (common.ID, error) {
	if err := r.teaData.Delete(ctx, uuid.UUID(id)); err != nil {
		return common.ID{}, err
	}
	return id, nil
}

// WriteToQR is the resolver for the writeToQR field.
func (r *mutationResolver) WriteToQR(context.Context, common.ID, model.QRRecordData) (*model.QRRecord, error) {
	panic(fmt.Errorf("not implemented: WriteToQR - writeToQR"))
}

// CreateTagCategory is the resolver for the createTagCategory field.
func (r *mutationResolver) CreateTagCategory(ctx context.Context, name string) (*model.TagCategory, error) {
	category, err := r.tagManager.CreateCategory(ctx, name)
	if err != nil {
		return nil, err
	}
	return &model.TagCategory{
		ID:   common.ID(category.ID),
		Name: category.Name,
	}, nil
}

// UpdateTagCategory is the resolver for the updateTagCategory field.
func (r *mutationResolver) UpdateTagCategory(ctx context.Context, id common.ID, name string) (*model.TagCategory, error) {
	cat, err := r.tagManager.UpdateCategory(ctx, uuid.UUID(id), name)
	if err != nil {
		return nil, err
	}
	return &model.TagCategory{
		ID:   common.ID(cat.ID),
		Name: cat.Name,
	}, nil
}

// DeleteTagCategory is the resolver for the deleteTagCategory field.
func (r *mutationResolver) DeleteTagCategory(ctx context.Context, id common.ID) (common.ID, error) {
	if err := r.tagManager.DeleteCategory(ctx, uuid.UUID(id)); err != nil {
		return common.ID{}, err
	}
	return id, nil
}

// CreateTag is the resolver for the createTag field.
func (r *mutationResolver) CreateTag(ctx context.Context, name string, color string, category common.ID) (*model.Tag, error) {
	tag, err := r.tagManager.Create(ctx, name, color, uuid.UUID(category))
	if err != nil {
		return nil, err
	}
	return &model.Tag{
		ID:    common.ID(tag.ID),
		Name:  tag.Name,
		Color: tag.Color,
	}, nil
}

// UpdateTag is the resolver for the updateTag field.
func (r *mutationResolver) UpdateTag(ctx context.Context, id common.ID, name string, color string) (*model.Tag, error) {
	tag, err := r.tagManager.Update(ctx, uuid.UUID(id), name, color)
	if err != nil {
		return nil, err
	}
	return &model.Tag{
		ID:    common.ID(tag.ID),
		Name:  tag.Name,
		Color: tag.Color,
	}, nil
}

// ChangeTagCategory is the resolver for the changeTagCategory field.
func (r *mutationResolver) ChangeTagCategory(ctx context.Context, id common.ID, category common.ID) (*model.Tag, error) {
	tag, err := r.tagManager.ChangeCategory(ctx, uuid.UUID(id), uuid.UUID(category))
	if err != nil {
		return nil, err
	}
	return &model.Tag{
		ID:    common.ID(tag.ID),
		Name:  tag.Name,
		Color: tag.Color,
	}, nil
}

// DeleteTag is the resolver for the deleteTag field.
func (r *mutationResolver) DeleteTag(ctx context.Context, id common.ID) (common.ID, error) {
	if err := r.tagManager.Delete(ctx, uuid.UUID(id)); err != nil {
		return [16]byte{}, err
	}
	return id, nil
}

// Teas is the resolver for the teas field.
func (r *queryResolver) Teas(ctx context.Context, prefix *string) ([]*model.Tea, error) {
	res, err := r.teaData.List(ctx, prefix)
	if err != nil {
		return nil, err
	}
	data := make([]*model.Tea, len(res))
	for i, el := range res {
		data[i] = model.FromCommonTea(&el)
	}
	return data, nil
}

// Tea is the resolver for the tea field.
func (r *queryResolver) Tea(ctx context.Context, id common.ID) (*model.Tea, error) {
	res, err := r.teaData.Get(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	return model.FromCommonTea(res), nil
}

// QRRecord is the resolver for the qrRecord field.
func (r *queryResolver) QRRecord(context.Context, common.ID) (*model.QRRecord, error) {
	panic(fmt.Errorf("not implemented: QRRecord - qrRecord"))
}

// Tag is the resolver for the tag field.
func (r *queryResolver) Tag(ctx context.Context, id common.ID) (*model.Tag, error) {
	tag, err := r.tagManager.Get(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	return &model.Tag{
		ID:    common.ID(tag.ID),
		Name:  tag.Name,
		Color: tag.Color,
		Category: &model.TagCategory{
			ID: common.ID(tag.CategoryID),
		},
	}, nil
}

// TagsCategories is the resolver for the tagsCategories field.
func (r *queryResolver) TagsCategories(ctx context.Context, name *string) ([]*model.TagCategory, error) {
	categories, err := r.tagManager.ListCategory(ctx, name)
	if err != nil {
		return nil, err
	}
	result := make([]*model.TagCategory, len(categories))
	for i, cat := range categories {
		result[i] = &model.TagCategory{
			ID:   common.ID(cat.ID),
			Name: cat.Name,
		}
	}
	return result, nil
}

// GetTeas is the resolver for the getTeas field.
func (r *queryResolver) GetTeas(ctx context.Context, prefix *string) ([]*model.Tea, error) {
	res, err := r.teaData.List(ctx, prefix)
	if err != nil {
		return nil, err
	}
	data := make([]*model.Tea, len(res))
	for i, el := range res {
		data[i] = model.FromCommonTea(&el)
	}
	return data, nil
}

// GetTea is the resolver for the getTea field.
func (r *queryResolver) GetTea(ctx context.Context, id common.ID) (*model.Tea, error) {
	res, err := r.teaData.Get(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	return model.FromCommonTea(res), nil
}

// GetQRRecord is the resolver for the getQrRecord field.
func (r *queryResolver) GetQRRecord(context.Context, common.ID) (*model.QRRecord, error) {
	panic(fmt.Errorf("not implemented: GetQRRecord - getQrRecord"))
}

// GetTags is the resolver for the getTags field.
func (r *queryResolver) GetTags(ctx context.Context, name *string, category *common.ID) ([]*model.Tag, error) {
	var cat *uuid.UUID
	if category != nil {
		cat = (*uuid.UUID)(category)
	}
	tags, err := r.tagManager.List(ctx, name, cat)
	if err != nil {
		return nil, err
	}
	result := make([]*model.Tag, len(tags))
	if len(tags) == 0 {
		return result, nil
	}
	categories, err := r.tagManager.ListCategory(ctx, nil)
	if err != nil {
		return nil, err
	}
	catMap := map[uuid.UUID]*model.TagCategory{}
	for _, ctg := range categories {
		catMap[ctg.ID] = &model.TagCategory{
			ID:   common.ID(ctg.ID),
			Name: ctg.Name,
		}
	}
	for i, tag := range tags {
		result[i] = &model.Tag{
			ID:       common.ID(tag.ID),
			Name:     tag.Name,
			Color:    tag.Color,
			Category: catMap[tag.CategoryID],
		}
	}
	return result, nil
}

// GetTagsCategories is the resolver for the getTagsCategories field.
func (r *queryResolver) GetTagsCategories(ctx context.Context, name *string) ([]*model.TagCategory, error) {
	categories, err := r.tagManager.ListCategory(ctx, name)
	if err != nil {
		return nil, err
	}
	result := make([]*model.TagCategory, len(categories))
	for i, cat := range categories {
		result[i] = &model.TagCategory{
			ID:   common.ID(cat.ID),
			Name: cat.Name,
		}
	}
	return result, nil
}

// GetTag is the resolver for the getTag field.
func (r *queryResolver) GetTag(ctx context.Context, id common.ID) (*model.Tag, error) {
	tag, err := r.tagManager.Get(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	return &model.Tag{
		ID:    common.ID(tag.ID),
		Name:  tag.Name,
		Color: tag.Color,
		Category: &model.TagCategory{
			ID: common.ID(tag.CategoryID),
		},
	}, nil
}

// OnCreateTea is the resolver for the onCreateTea field.
func (r *subscriptionResolver) OnCreateTea(ctx context.Context) (<-chan *model.Tea, error) {
	return r.teaData.SubscribeOnCreate(ctx)
}

// OnUpdateTea is the resolver for the onUpdateTea field.
func (r *subscriptionResolver) OnUpdateTea(ctx context.Context) (<-chan *model.Tea, error) {
	r.log.Debug("subscribe on update")
	return r.teaData.SubscribeOnUpdate(ctx)
}

// OnDeleteTea is the resolver for the onDeleteTea field.
func (r *subscriptionResolver) OnDeleteTea(ctx context.Context) (<-chan common.ID, error) {
	return r.teaData.SubscribeOnDelete(ctx)
}

// OnCreateTagCategory is the resolver for the onCreateTagCategory field.
func (r *subscriptionResolver) OnCreateTagCategory(ctx context.Context) (<-chan *model.TagCategory, error) {
	return r.tagManager.SubscribeOnCreateCategory(ctx)
}

// OnUpdateTagCategory is the resolver for the onUpdateTagCategory field.
func (r *subscriptionResolver) OnUpdateTagCategory(ctx context.Context) (<-chan *model.TagCategory, error) {
	return r.tagManager.SubscribeOnUpdateCategory(ctx)
}

// OnDeleteTagCategory is the resolver for the onDeleteTagCategory field.
func (r *subscriptionResolver) OnDeleteTagCategory(ctx context.Context) (<-chan common.ID, error) {
	return r.tagManager.SubscribeOnDeleteCategory(ctx)
}

// OnCreateTag is the resolver for the onCreateTag field.
func (r *subscriptionResolver) OnCreateTag(ctx context.Context) (<-chan *model.Tag, error) {
	return r.tagManager.SubscribeOnCreate(ctx)
}

// OnUpdateTag is the resolver for the onUpdateTag field.
func (r *subscriptionResolver) OnUpdateTag(ctx context.Context) (<-chan *model.Tag, error) {
	return r.tagManager.SubscribeOnUpdate(ctx)
}

// OnDeleteTag is the resolver for the onDeleteTag field.
func (r *subscriptionResolver) OnDeleteTag(ctx context.Context) (<-chan common.ID, error) {
	return r.tagManager.SubscribeOnDelete(ctx)
}

// OnAddTagToTea is the resolver for the onAddTagToTea field.
func (r *subscriptionResolver) OnAddTagToTea(ctx context.Context) (<-chan *model.Tea, error) {
	return r.tagManager.SubscribeOnAddTagToTea(ctx)
}

// OnDeleteTagFromTea is the resolver for the onDeleteTagFromTea field.
func (r *subscriptionResolver) OnDeleteTagFromTea(ctx context.Context) (<-chan *model.Tea, error) {
	return r.tagManager.SubscribeOnDeleteTagToTea(ctx)
}

// Category is the resolver for the category field.
func (r *tagResolver) Category(ctx context.Context, obj *model.Tag) (*model.TagCategory, error) {
	if obj.Category == nil {
		return nil, nil
	}
	if obj.Category.Name != "" {
		return obj.Category, nil
	}
	cat, err := r.tagManager.GetCategory(ctx, uuid.UUID(obj.Category.ID))
	if err != nil {
		return nil, err
	}
	return &model.TagCategory{
		ID:   common.ID(cat.ID),
		Name: cat.Name,
	}, nil
}

// Tags is the resolver for the tags field.
func (r *tagCategoryResolver) Tags(ctx context.Context, obj *model.TagCategory, name *string) ([]*model.Tag, error) {
	var cat uuid.UUID
	if obj != nil {
		cat = uuid.UUID(obj.ID)
	}
	tags, err := r.tagManager.List(ctx, name, &cat)
	if err != nil {
		return nil, err
	}
	result := make([]*model.Tag, len(tags))
	if len(tags) == 0 {
		return result, nil
	}
	categories, err := r.tagManager.ListCategory(ctx, nil)
	if err != nil {
		return nil, err
	}
	catMap := map[uuid.UUID]*model.TagCategory{}
	for _, ctg := range categories {
		catMap[ctg.ID] = &model.TagCategory{
			ID:   common.ID(ctg.ID),
			Name: ctg.Name,
		}
	}
	for i, tag := range tags {
		result[i] = &model.Tag{
			ID:       common.ID(tag.ID),
			Name:     tag.Name,
			Color:    tag.Color,
			Category: catMap[tag.CategoryID],
		}
	}
	return result, nil
}

// Tags is the resolver for the tags field.
func (r *teaResolver) Tags(ctx context.Context, obj *model.Tea) ([]*model.Tag, error) {
	tags, err := r.tagManager.ListByTea(ctx, uuid.UUID(obj.ID))
	if err != nil {
		return nil, err
	}
	result := make([]*model.Tag, len(tags))
	for i, t := range tags {
		result[i] = &model.Tag{
			ID:       common.ID(t.ID),
			Name:     t.Name,
			Color:    t.Color,
			Category: &model.TagCategory{ID: common.ID(t.CategoryID)},
		}
	}
	return result, nil
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

// Subscription returns generated.SubscriptionResolver implementation.
func (r *Resolver) Subscription() generated.SubscriptionResolver { return &subscriptionResolver{r} }

// Tag returns generated.TagResolver implementation.
func (r *Resolver) Tag() generated.TagResolver { return &tagResolver{r} }

// TagCategory returns generated.TagCategoryResolver implementation.
func (r *Resolver) TagCategory() generated.TagCategoryResolver { return &tagCategoryResolver{r} }

// Tea returns generated.TeaResolver implementation.
func (r *Resolver) Tea() generated.TeaResolver { return &teaResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }
type tagResolver struct{ *Resolver }
type tagCategoryResolver struct{ *Resolver }
type teaResolver struct{ *Resolver }

// !!! WARNING !!!
// The code below was going to be deleted when updating resolvers. It has been copied here so you have
// one last chance to move it out of harms way if you want. There are two reasons this happens:
//   - When renaming or deleting a resolver the old code will be put in here. You can safely delete
//     it when you're done.
//   - You have helper methods in this file. Move them out to keep these resolver files clean.
func (r *mutationResolver) WriteToQr(ctx context.Context, id common.ID, data model.QRRecordData) (*model.QRRecord, error) {
	if err := r.qrManager.Set(ctx, uuid.UUID(id), &data); err != nil {
		return nil, err
	}
	tea, err := r.teaData.Get(ctx, uuid.UUID(data.Tea))
	if err != nil {
		return nil, err
	}
	return &model.QRRecord{
		ID: id,
		Tea: &model.Tea{
			ID:          common.ID(tea.ID),
			Name:        tea.Name,
			Type:        model.Type(tea.Type),
			Description: tea.Description,
		},
		BowlingTemp:    data.BowlingTemp,
		ExpirationDate: data.ExpirationDate,
	}, nil
}
func (r *queryResolver) QrRecord(ctx context.Context, id common.ID) (*model.QRRecord, error) {
	data, err := r.qrManager.Get(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	tea, err := r.teaData.Get(ctx, uuid.UUID(data.Tea))
	if err != nil {
		return nil, err
	}
	return &model.QRRecord{
		ID: id,
		Tea: &model.Tea{
			ID:          common.ID(tea.ID),
			Name:        tea.Name,
			Type:        model.Type(tea.Type),
			Description: tea.Description,
		},
		BowlingTemp:    data.BowlingTemp,
		ExpirationDate: data.ExpirationDate,
	}, nil
}
func (r *queryResolver) GetQrRecord(ctx context.Context, id common.ID) (*model.QRRecord, error) {
	data, err := r.qrManager.Get(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	tea, err := r.teaData.Get(ctx, uuid.UUID(data.Tea))
	if err != nil {
		return nil, err
	}
	return &model.QRRecord{
		ID: id,
		Tea: &model.Tea{
			ID:          common.ID(tea.ID),
			Name:        tea.Name,
			Type:        model.Type(tea.Type),
			Description: tea.Description,
		},
		BowlingTemp:    data.BowlingTemp,
		ExpirationDate: data.ExpirationDate,
	}, nil
}
