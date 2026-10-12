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

var errCatalogueReceiptsUnavailable = errors.New("catalog operation receipts unavailable")

type catalogueStore interface {
	CatalogueCapabilities(ctx context.Context) (bool, error)
	CatalogueOperationReceipt(ctx context.Context, key uuid.UUID) (*common.CatalogueReceipt, error)
}

// SetCatalogueStore wires the versioned catalog prerequisite without changing old resolver clients.
func (r *Resolver) SetCatalogueStore(store catalogueStore) { r.catalog = store }

func catalogueMutationContext(ctx context.Context, revision, key *string) (context.Context, error) {
	actor := "admin"
	if user, err := authPkg.GetUser(ctx); err == nil {
		actor = "user:" + user.ID.String()
	}

	result, err := common.WithCatalogueOperation(ctx, revision, key, actor)
	if err != nil {
		return nil, fmt.Errorf("validate catalog operation: %w", err)
	}

	return result, nil
}

func (r *Resolver) editorCapabilities(ctx context.Context) (*model.EditorCapabilities, error) {
	if err := authPkg.RequireAdmin(ctx); err != nil {
		return nil, castGQLError(ctx, err)
	}

	ready := false

	if r.catalog != nil {
		var err error

		ready, err = r.catalog.CatalogueCapabilities(ctx)
		if err != nil {
			return nil, castGQLError(ctx, err)
		}
	}

	return &model.EditorCapabilities{ProtocolVersion: "1", SupportsRevisions: ready, SupportsIdempotency: ready}, nil
}

//nolint:nilnil // A missing optional receipt is GraphQL null, not a failed lookup.
func (r *Resolver) editorOperationReceipt(ctx context.Context, key string) (*model.CatalogueOperationReceipt, error) {
	if err := authPkg.RequireAdmin(ctx); err != nil {
		return nil, castGQLError(ctx, err)
	}

	id, err := uuid.Parse(key)
	if err != nil || id == uuid.Nil {
		return nil, castGQLError(ctx, common.ErrInvalidCatalogueOperation)
	}

	if r.catalog == nil {
		return nil, castGQLError(ctx, errCatalogueReceiptsUnavailable)
	}

	receipt, err := r.catalog.CatalogueOperationReceipt(ctx, id)
	if err != nil {
		return nil, castGQLError(ctx, err)
	}

	if receipt == nil {
		return nil, nil
	}

	return &model.CatalogueOperationReceipt{IdempotencyKey: receipt.Key.String(), Operation: receipt.Operation, TeaID: gqlCommon.ID(receipt.TeaID), ResourceID: gqlCommon.ID(receipt.ResourceID), ResultRevision: receipt.ResultRevision, CompletedAt: receipt.CompletedAt}, nil
}
