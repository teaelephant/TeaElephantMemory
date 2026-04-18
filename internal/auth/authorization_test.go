package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
)

func TestRequireUserOrAdmin(t *testing.T) {
	t.Run("accepts user session", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), userCtxKey, &common.User{ID: uuid.New()})

		if err := RequireUserOrAdmin(ctx); err != nil {
			t.Fatalf("RequireUserOrAdmin returned error for user session: %v", err)
		}
	})

	t.Run("accepts admin session", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), adminCtxKey, &AdminPrincipal{JTI: uuid.NewString()})

		if err := RequireUserOrAdmin(ctx); err != nil {
			t.Fatalf("RequireUserOrAdmin returned error for admin session: %v", err)
		}
	})

	t.Run("rejects unauthenticated context", func(t *testing.T) {
		if err := RequireUserOrAdmin(context.Background()); err == nil {
			t.Fatal("RequireUserOrAdmin returned nil for unauthenticated context")
		}
	})
}
