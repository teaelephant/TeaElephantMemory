package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Timothylock/go-signin-with-apple/apple"
	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
)

const userCtxKey = "user"

type Auth interface {
	CheckToken(ctx context.Context, token string) (uuid.UUID, error)
	Middleware(http.Handler) http.Handler
	Start() error
}

type storage interface {
	GetOrCreateUser(ctx context.Context, unique string) (uuid.UUID, error)
}

type auth struct {
	appleClient *apple.Client
	cfg         *Configuration
	secret      string

	storage
}

func (a *auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		// Allow unauthenticated users in
		if header == "" {
			next.ServeHTTP(w, r)
			return
		}

		token := strings.Replace(header, "Bearer ", "", 1)

		userId, err := a.CheckToken(r.Context(), token)
		if err != nil {
			http.Error(w, "Invalid cookie", http.StatusForbidden)
			return
		}

		// and call the next with our new context
		r = r.WithContext(context.WithValue(r.Context(), userCtxKey, &common.User{
			ID: userId,
		}))
		next.ServeHTTP(w, r)
	})
}

func (a *auth) Start() (err error) {
	a.secret, err = apple.GenerateClientSecret(a.cfg.Secret, a.cfg.TeamID, a.cfg.ClientID, a.cfg.KeyID)
	return err
}

func (a *auth) CheckToken(ctx context.Context, token string) (uuid.UUID, error) {
	vReq := apple.WebValidationTokenRequest{
		ClientID:     a.cfg.ClientID,
		ClientSecret: a.secret,
		Code:         token,
	}

	var resp apple.ValidationResponse

	// Do the verification
	if err := a.appleClient.VerifyWebToken(ctx, vReq, &resp); err != nil {
		return uuid.UUID{}, err
	}

	if resp.Error != "" {
		return uuid.UUID{}, errors.New(resp.ErrorDescription)
	}

	unique, err := apple.GetUniqueID(resp.IDToken)
	if err != nil {
		return uuid.UUID{}, err
	}

	return a.storage.GetOrCreateUser(ctx, unique)
}

func NewAuth(cfg *Configuration, storage storage) Auth {
	return &auth{cfg: cfg, appleClient: apple.New(), storage: storage}
}

func GetUser(ctx context.Context) (*common.User, error) {
	value := ctx.Value(userCtxKey)
	if user, ok := (value).(*common.User); ok {
		return user, nil
	}

	return nil, common.ErrUserNotFound
}
