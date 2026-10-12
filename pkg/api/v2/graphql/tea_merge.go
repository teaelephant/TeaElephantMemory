package graphql

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/teaelephant/TeaElephantMemory/common"
	authPkg "github.com/teaelephant/TeaElephantMemory/internal/auth"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

type teaMergeStore interface {
	TeaMergeCapabilities(ctx context.Context) (bool, error)
	PreviewTeaMerge(ctx context.Context, source, target uuid.UUID) (*common.TeaMergePreview, error)
	MergeTeas(ctx context.Context, command common.TeaMergeCommand) (*common.TeaMergeResult, error)
	TeaMergeResultForReceipt(ctx context.Context, command common.TeaMergeCommand) (*common.TeaMergeResult, error)
}

type teaMergeNotifier interface {
	NotifyMerge(ctx context.Context, source uuid.UUID, target *common.Tea) error
}

func (r *Resolver) requireTeaMerge(ctx context.Context) error {
	if err := authPkg.RequireAdmin(ctx); err != nil {
		return fmt.Errorf("authorize Tea merge: %w", err)
	}

	if r.merges == nil || r.catalog == nil {
		return common.ErrTeaMergeUnavailable
	}

	capabilities, err := r.editorCapabilities(ctx)
	if err != nil {
		return fmt.Errorf("read Tea merge capabilities: %w", err)
	}

	if !capabilities.SupportsTeaMerge {
		return common.ErrTeaMergeUnavailable
	}

	return nil
}

func (r *Resolver) previewTeaMerge(ctx context.Context, source, target gqlCommon.ID) (*model.TeaMergePreview, error) {
	if err := r.requireTeaMerge(ctx); err != nil {
		return nil, castGQLError(ctx, err)
	}

	preview, err := r.merges.PreviewTeaMerge(ctx, uuid.UUID(source), uuid.UUID(target))
	if err != nil {
		return nil, castGQLError(ctx, err)
	}

	value := &model.TeaMergePreview{
		SourceTea: mergeSnapshotModel(preview.Source), TargetTea: mergeSnapshotModel(preview.Target), PreviewToken: preview.Token,
		QRCount: preview.Counts.QRCount, CollectionMembershipCount: preview.Counts.CollectionMembershipCount,
		TagsToAdd: preview.Counts.TagsAdded, ImagesToMove: preview.Counts.ImagesMoved, ConsumptionCount: preview.Counts.ConsumptionsMoved,
		ResultingImageCount: preview.ResultingImageCount, Blockers: preview.Blockers,
		MetadataPolicy: common.TeaMergeMetadataPolicy, CoverPolicy: common.TeaMergeCoverPolicy,
	}
	if preview.ResultingCoverImageID != nil {
		id := gqlCommon.ID(*preview.ResultingCoverImageID)
		value.ResultingCoverImageID = &id
	}

	return value, nil
}

func (r *Resolver) mergeTeas(ctx context.Context, source, target gqlCommon.ID, sourceRevision, targetRevision, token, key string) (*model.TeaMergeResult, error) {
	if err := r.requireTeaMerge(ctx); err != nil {
		return nil, castGQLError(ctx, err)
	}

	operationCtx, err := catalogueMutationContext(ctx, nil, &key)
	if err != nil {
		return nil, castGQLError(ctx, err)
	}

	ctx = operationCtx

	result, err := r.merges.MergeTeas(ctx, common.TeaMergeCommand{
		SourceTeaID: uuid.UUID(source), TargetTeaID: uuid.UUID(target), ExpectedSourceRevision: sourceRevision,
		ExpectedTargetRevision: targetRevision, PreviewToken: token,
	})
	if err != nil {
		return nil, castGQLError(ctx, err)
	}

	if !result.Replayed {
		if notifier, ok := r.teaData.(teaMergeNotifier); ok {
			if err := notifier.NotifyMerge(ctx, result.Receipt.ResourceID, result.Target); err != nil {
				return nil, castGQLError(ctx, err)
			}
		}
	}

	return mergeResultModel(result), nil
}

//nolint:nilnil // A missing committed merge is an optional GraphQL null.
func (r *Resolver) editorTeaMergeReceipt(ctx context.Context, source, target gqlCommon.ID, sourceRevision, targetRevision, token, key string) (*model.TeaMergeResult, error) {
	if err := r.requireTeaMerge(ctx); err != nil {
		return nil, castGQLError(ctx, err)
	}

	operationCtx, err := catalogueMutationContext(ctx, nil, &key)
	if err != nil {
		return nil, castGQLError(ctx, err)
	}

	result, err := r.merges.TeaMergeResultForReceipt(operationCtx, common.TeaMergeCommand{
		SourceTeaID: uuid.UUID(source), TargetTeaID: uuid.UUID(target), ExpectedSourceRevision: sourceRevision,
		ExpectedTargetRevision: targetRevision, PreviewToken: token,
	})
	if errors.Is(err, common.ErrTeaNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, castGQLError(ctx, err)
	}

	return mergeResultModel(result), nil
}

func mergeResultModel(result *common.TeaMergeResult) *model.TeaMergeResult {
	receipt := result.Receipt

	value := &model.TeaMergeResult{
		Receipt: &model.CatalogueOperationReceipt{IdempotencyKey: receipt.Key.String(), Operation: receipt.Operation,
			TeaID: gqlCommon.ID(receipt.TeaID), ResourceID: gqlCommon.ID(receipt.ResourceID), ResultRevision: receipt.ResultRevision, CompletedAt: receipt.CompletedAt},
		Replayed: result.Replayed, SourceTeaID: gqlCommon.ID(receipt.ResourceID), TargetTeaID: gqlCommon.ID(receipt.TeaID),
		QRCount: result.Counts.QRCount, CollectionMembershipCount: result.Counts.CollectionMembershipCount,
		TagsAdded: result.Counts.TagsAdded, ImagesMoved: result.Counts.ImagesMoved, ConsumptionsMoved: result.Counts.ConsumptionsMoved,
	}
	if result.TargetSnapshot != nil {
		value.TargetTea = mergeSnapshotModel(*result.TargetSnapshot)
	}

	return value
}

func mergeSnapshotModel(snapshot common.TeaMergeSnapshot) *model.Tea {
	tea := model.FromCommonTea(snapshot.Tea)

	tea.Tags = make([]*model.Tag, 0, len(snapshot.Tags))
	for _, value := range snapshot.Tags {
		item, category := value.Tag, value.Category
		tea.Tags = append(tea.Tags, &model.Tag{ID: gqlCommon.ID(item.ID), Name: item.Name, Color: item.Color, Revision: item.Revision,
			Category: &model.TagCategory{ID: gqlCommon.ID(category.ID), Name: category.Name, Revision: category.Revision}})
	}

	tea.Images = make([]*model.TeaImage, 0, len(snapshot.Images))
	for _, image := range snapshot.Images {
		tea.Images = append(tea.Images, &model.TeaImage{ID: gqlCommon.ID(image.ID), URL: "/v2/tea-images/" + image.ID.String(),
			ContentType: image.ContentType, Width: image.Width, Height: image.Height, CreatedAt: image.CreatedAt, IsCover: image.IsCover})
	}

	return tea
}
