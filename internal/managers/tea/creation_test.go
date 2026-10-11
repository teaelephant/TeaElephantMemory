// Package tea manages shared catalog entries and their update streams.
package tea

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/teaelephant/TeaElephantMemory/common"
)

const creationTeaName = "New tea"

type createQueueStorage struct {
	storage
	record    *common.Tea
	tagIDs    []uuid.UUID
	failure   error
	started   chan struct{}
	committed chan struct{}
}

func (s *createQueueStorage) WriteRecordWithTags(_ context.Context, _ *common.TeaData, tagIDs []uuid.UUID) (*common.Tea, error) {
	s.tagIDs = tagIDs
	if s.started != nil {
		close(s.started)
		<-s.committed
	}

	return s.record, s.failure
}

func (s *createQueueStorage) WriteRecord(context.Context, *common.TeaData) (*common.Tea, error) {
	return s.record, nil
}

func TestCreatePublishesNewTeaToCreationQueue(t *testing.T) {
	t.Parallel()

	record := &common.Tea{ID: uuid.New(), TeaData: &common.TeaData{Name: creationTeaName, Type: common.TeaBeverageType}}
	subject, ok := NewManager(&createQueueStorage{record: record}).(*manager)
	require.True(t, ok)
	created, err := subject.Create(t.Context(), record.TeaData)
	require.NoError(t, err)
	require.Equal(t, record.ID, created.ID)

	select {
	case event := <-subject.create:
		require.Equal(t, record.ID, event.ID)
		require.Equal(t, creationTeaName, event.Name)
	default:
		t.Fatal("successful creation must publish the new Tea to the creation queue")
	}
}

func TestCreateWithTagsPublishesOnlyAfterStorageCommit(t *testing.T) {
	t.Parallel()

	record := &common.Tea{ID: uuid.New(), TeaData: &common.TeaData{Name: creationTeaName, Type: common.TeaBeverageType}}
	store := &createQueueStorage{record: record, started: make(chan struct{}), committed: make(chan struct{})}
	subject, ok := NewManager(store).(*manager)
	require.True(t, ok)

	tags := []uuid.UUID{uuid.New()}
	done := make(chan error, 1)

	go func() {
		_, err := subject.CreateWithTags(t.Context(), record.TeaData, tags)
		done <- err
	}()

	<-store.started
	require.Empty(t, subject.create, "creation must not be announced before the storage transaction commits")
	close(store.committed)
	require.NoError(t, <-done)
	require.Equal(t, tags, store.tagIDs)

	select {
	case event := <-subject.create:
		require.Equal(t, record.ID, event.ID)
	default:
		t.Fatal("committed creation must publish the new Tea")
	}
}

func TestCreateWithTagsDoesNotPublishFailedTransactions(t *testing.T) {
	t.Parallel()

	for _, failure := range []error{common.ErrTagNotFound, sql.ErrTxDone} {
		t.Run(failure.Error(), func(t *testing.T) {
			store := &createQueueStorage{failure: failure}
			subject, ok := NewManager(store).(*manager)
			require.True(t, ok)
			created, err := subject.CreateWithTags(t.Context(), &common.TeaData{Name: creationTeaName}, []uuid.UUID{uuid.New()})
			require.ErrorIs(t, err, failure)
			require.Nil(t, created)
			require.Empty(t, subject.create, "failed transaction must not publish a creation event")
		})
	}
}
