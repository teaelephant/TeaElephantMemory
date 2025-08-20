package consumption

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
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
