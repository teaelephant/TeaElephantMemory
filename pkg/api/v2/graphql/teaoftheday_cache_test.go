package graphql

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

func TestTeaOfTheDayCache_GetSetAndExpire(t *testing.T) {
	c := newTeaOfTheDayCache()
	userID := uuid.New()

	// Fixed time for deterministic test
	now := time.Date(2025, 8, 21, 10, 0, 0, 0, time.Local)

	// Initially empty
	if v, ok := c.Get(userID, now); ok || v != nil {
		t.Fatalf("expected empty cache")
	}

	val := &model.TeaOfTheDay{Tea: nil, Date: now}
	c.Set(userID, val, now)

	// Should hit before midnight
	v, ok := c.Get(userID, now.Add(1*time.Hour))
	require.True(t, ok)
	require.Equal(t, val, v)

	// After next midnight it should expire
	after := nextMidnight(now)
	v2, ok2 := c.Get(userID, after)
	require.False(t, ok2)
	require.Nil(t, v2)
}
