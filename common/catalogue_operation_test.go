package common

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

const operationTestActor = "admin"

func TestCatalogueMutationOptionsRejectMalformedPreconditions(t *testing.T) {
	for _, value := range []string{"", "0", "-1", "01", "9223372036854775808", "not-a-version"} {
		_, err := WithCatalogueOperation(context.Background(), &value, nil, operationTestActor)
		require.ErrorIs(t, err, ErrInvalidCatalogueOperation)
	}

	for _, value := range []string{"", "not-a-key", uuid.Nil.String()} {
		_, err := WithCatalogueOperation(context.Background(), nil, &value, operationTestActor)
		require.ErrorIs(t, err, ErrInvalidCatalogueOperation)
	}

	revision, key := "42", uuid.NewString()
	ctx, err := WithCatalogueOperation(context.Background(), &revision, &key, operationTestActor)
	require.NoError(t, err)

	options := CatalogueOperationFrom(ctx)
	require.Equal(t, revision, options.ExpectedRevision)
	require.True(t, options.HasExpectedRevision)
	require.True(t, options.HasKey)
	require.Equal(t, key, options.Key.String())
	require.Equal(t, operationTestActor, options.Actor)

	untouched := CatalogueOperationFrom(context.Background())
	require.False(t, untouched.HasExpectedRevision)
	require.False(t, untouched.HasKey)
}
