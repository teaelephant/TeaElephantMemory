package graphql

import (
	"sync"
	"time"

	"github.com/google/uuid"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

// teaOfTheDayCache is a simple in-memory cache for TeaOfTheDay per user
// that expires at the next local midnight. It is process-local and not shared.
//
// Day boundary is computed using the provided time's location (typically time.Local),
// which is sufficient given we do not have per-user timezone in the system yet.
// The cache intentionally does not attempt cross-process or cross-instance sharing.
// It is safe for concurrent use by multiple goroutines.
type teaOfTheDayCache struct {
	mu    sync.RWMutex
	items map[uuid.UUID]cachedTeaOfTheDay
}

type cachedTeaOfTheDay struct {
	val       *model.TeaOfTheDay
	expiresAt time.Time
}

func newTeaOfTheDayCache() *teaOfTheDayCache {
	return &teaOfTheDayCache{items: make(map[uuid.UUID]cachedTeaOfTheDay)}
}

// nextMidnight returns the next midnight in the same location as t.
func nextMidnight(t time.Time) time.Time {
	y, m, d := t.In(t.Location()).Date()
	loc := t.Location()

	return time.Date(y, m, d+1, 0, 0, 0, 0, loc)
}

// Get returns the cached value for userID if present and not expired at now.
func (c *teaOfTheDayCache) Get(userID uuid.UUID, now time.Time) (*model.TeaOfTheDay, bool) {
	c.mu.RLock()
	entry, ok := c.items[userID]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if !now.Before(entry.expiresAt) {
		// expired; delete lazily
		c.mu.Lock()
		// recheck in case another goroutine updated it
		entry2, ok2 := c.items[userID]
		if ok2 && !now.Before(entry2.expiresAt) {
			delete(c.items, userID)
		}

		c.mu.Unlock()

		return nil, false
	}

	return entry.val, true
}

// Set stores the value for userID, expiring at the next local midnight based on now.
func (c *teaOfTheDayCache) Set(userID uuid.UUID, v *model.TeaOfTheDay, now time.Time) {
	exp := nextMidnight(now)

	c.mu.Lock()
	c.items[userID] = cachedTeaOfTheDay{val: v, expiresAt: exp}
	c.mu.Unlock()
}
