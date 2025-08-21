package consumption

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	foundationdb "github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common/key_value/key_builder"
	"github.com/teaelephant/TeaElephantMemory/pkg/fdbclient"
)

// Consumption represents a single tea consumption event.
type Consumption struct {
	TeaID uuid.UUID
	Time  time.Time
}

// Store defines operations for recording and querying recent tea consumption.
type Store interface {
	Record(ctx context.Context, userID uuid.UUID, teaID uuid.UUID, ts time.Time) error
	// Recent returns consumptions since the provided time (inclusive) for the given user.
	Recent(ctx context.Context, userID uuid.UUID, since time.Time) ([]Consumption, error)
}

// MemoryStore is a simple in-memory implementation of Store.
// It is process-local and not persisted across restarts.
type MemoryStore struct {
	mu   sync.Mutex
	data map[uuid.UUID][]Consumption // userID -> consumptions (unsorted)
	// Optional retention to avoid unbounded growth
	retention time.Duration
}

func NewMemoryStore(retention time.Duration) *MemoryStore {
	if retention <= 0 {
		retention = 30 * 24 * time.Hour // default 30 days
	}

	return &MemoryStore{data: make(map[uuid.UUID][]Consumption), retention: retention}
}

func (m *MemoryStore) Record(_ context.Context, userID uuid.UUID, teaID uuid.UUID, ts time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Append record
	m.data[userID] = append(m.data[userID], Consumption{TeaID: teaID, Time: ts})

	// Retain only events within retention window
	cutoff := ts.Add(-m.retention)
	events := m.data[userID]

	filtered := events[:0]
	for _, e := range events {
		if e.Time.After(cutoff) {
			filtered = append(filtered, e)
		}
	}
	// Copy to avoid aliasing if needed
	m.data[userID] = append([]Consumption(nil), filtered...)

	return nil
}

func (m *MemoryStore) Recent(_ context.Context, userID uuid.UUID, since time.Time) ([]Consumption, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	events := m.data[userID]

	out := make([]Consumption, 0, len(events))
	for _, e := range events {
		if !e.Time.Before(since) {
			out = append(out, e)
		}
	}
	// Sort by time desc (most recent first)
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })

	return out, nil
}

// FDBStore is a FoundationDB-backed implementation of Store.
type FDBStore struct {
	db        fdbclient.Database
	kb        key_builder.Builder
	retention time.Duration
}

// NewFDBStore creates a FoundationDB-backed consumption store with the given retention window.
// If retention <= 0, a default of 30 days is used.
func NewFDBStore(db fdbclient.Database, retention time.Duration) *FDBStore {
	if retention <= 0 {
		retention = 30 * 24 * time.Hour // default 30 days
	}

	return &FDBStore{db: db, kb: key_builder.NewBuilder(), retention: retention}
}

// Record writes a consumption entry for the given user and tea at the provided time.
// It also trims keys older than the retention window.
func (s *FDBStore) Record(ctx context.Context, userID uuid.UUID, teaID uuid.UUID, ts time.Time) error {
	tr, err := s.db.NewTransaction(ctx)
	if err != nil {
		return fmt.Errorf("consumption.Record: begin transaction: %w", err)
	}

	key := s.kb.ConsumptionKey(userID, ts, teaID)

	// store empty value; key encodes all information needed
	tr.Set(key, nil)

	// retention trimming: clear all keys older than cutoff
	cutoff := ts.Add(-s.retention)
	prefix := s.kb.ConsumptionByUserID(userID)

	pr, err := foundationdb.PrefixRange(prefix)
	if err != nil {
		return fmt.Errorf("consumption.Record: prefix range: %w", err)
	}

	it := tr.GetIterator(pr)
	for it.Advance() {
		kv, err := it.Get()
		if err != nil {
			return fmt.Errorf("consumption.Record: iterator get: %w", err)
		}
		// parse timestamp via key_builder
		kt, _, ok := key_builder.ParseConsumptionKey(prefix, kv.Key)
		if !ok {
			continue
		}

		if kt.Before(cutoff) {
			tr.Clear(kv.Key)
			continue
		}

		break // earliest key >= cutoff reached
	}

	if err := tr.Commit(); err != nil {
		return fmt.Errorf("consumption.Record: commit: %w", err)
	}

	return nil
}

// Recent returns consumption events for userID since the given time (inclusive),
// ordered from most recent to oldest.
func (s *FDBStore) Recent(ctx context.Context, userID uuid.UUID, since time.Time) ([]Consumption, error) {
	tr, err := s.db.NewTransaction(ctx)
	if err != nil {
		return nil, fmt.Errorf("consumption.Recent: begin transaction: %w", err)
	}

	prefix := s.kb.ConsumptionByUserID(userID)

	pr, err := foundationdb.PrefixRange(prefix)
	if err != nil {
		return nil, fmt.Errorf("consumption.Recent: prefix range: %w", err)
	}

	opts := &fdbclient.RangeOptions{}
	opts.SetReverse() // newest first

	it := tr.GetIterator(pr, opts)

	out := make([]Consumption, 0, 32)

	for it.Advance() {
		kv, err := it.Get()
		if err != nil {
			return nil, fmt.Errorf("consumption.Recent: iterator get: %w", err)
		}

		ts, teaID, ok := key_builder.ParseConsumptionKey(prefix, kv.Key)
		if !ok {
			continue
		}

		if ts.Before(since) {
			break // since cutoff reached in reverse order
		}

		out = append(out, Consumption{TeaID: teaID, Time: ts})
	}

	// ensure sorted by time desc (iterator already reverse, but keep invariant)
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })

	return out, nil
}
