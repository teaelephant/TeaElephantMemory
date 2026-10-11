package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
)

func TestRequireAdminDistinguishesMissingAndInsufficientAuthentication(t *testing.T) {
	for _, test := range []struct {
		name  string
		key   ctxKey
		value any
		want  error
	}{
		{name: "anonymous", want: common.ErrUnauthorized},
		{name: "user", key: userCtxKey, value: &common.User{ID: uuid.New()}, want: common.ErrNotAdmin},
		{name: "admin", key: adminCtxKey, value: &AdminPrincipal{JTI: uuid.NewString()}},
		{name: "nil admin", key: adminCtxKey, value: (*AdminPrincipal)(nil), want: common.ErrUnauthorized},
		{name: "nil user", key: userCtxKey, value: (*common.User)(nil), want: common.ErrUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			if test.key != "" {
				ctx = context.WithValue(ctx, test.key, test.value)
			}

			if err := RequireAdmin(ctx); !errors.Is(err, test.want) {
				t.Fatalf("RequireAdmin = %v, want %v", err, test.want)
			}
		})
	}
}

func TestGetUserRejectsNilUser(t *testing.T) {
	ctx := context.WithValue(context.Background(), userCtxKey, (*common.User)(nil))

	user, err := GetUser(ctx)
	if user != nil || !errors.Is(err, common.ErrUserNotFound) {
		t.Fatalf("GetUser = %v, %v; want nil, ErrUserNotFound", user, err)
	}
}

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
