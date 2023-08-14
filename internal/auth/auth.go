package auth

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Timothylock/go-signin-with-apple/apple"
	"github.com/golang-jwt/jwt/v5"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/common"
)

const (
	userCtxKey      = "user"
	JwtDurationHour = 24
)

var signingMethod = jwt.SigningMethodES256

type Auth interface {
	Auth(ctx context.Context, token string) (*common.Session, error)
	Validate(ctx context.Context, jwt string) (*common.User, error)
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
	log *logrus.Entry
}

func (a *auth) Validate(_ context.Context, jwtToken string) (*common.User, error) {
	result, err := jwt.Parse(jwtToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		// hmacSampleSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
		return a.cfg.Secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := result.Claims.(jwt.MapClaims)
	if !ok || !result.Valid {
		return nil, common.ErrInvalidToken
	}

	exp, err := claims.GetExpirationTime()
	if err != nil {
		return nil, err
	}

	if time.Now().After(exp.Time) {
		return nil, common.ErrExpiredToken
	}

	userIDStr, err := claims.GetIssuer()
	if err != nil {
		return nil, err
	}

	userID, err := uuid.FromString(userIDStr)
	if err != nil {
		return nil, err
	}

	return &common.User{
		// todo read from storage full user
		ID: userID,
	}, nil
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

		user, err := a.Validate(r.Context(), token)
		if err != nil {
			http.Error(w, "Invalid cookie", http.StatusForbidden)
			return
		}

		// and call the next with our new context
		r = r.WithContext(context.WithValue(r.Context(), userCtxKey, user))
		next.ServeHTTP(w, r)
	})
}

func (a *auth) Start() (err error) {
	a.secret, err = apple.GenerateClientSecret(a.cfg.Secret, a.cfg.TeamID, a.cfg.ClientID, a.cfg.KeyID)
	return err
}

func (a *auth) Auth(ctx context.Context, token string) (*common.Session, error) {
	vReq := apple.AppValidationTokenRequest{
		ClientID:     a.cfg.ClientID,
		ClientSecret: a.secret,
		Code:         token,
	}

	var resp apple.ValidationResponse

	// Do the verification
	if err := a.appleClient.VerifyAppToken(ctx, vReq, &resp); err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, errors.New(resp.ErrorDescription)
	}

	claims, err := apple.GetClaims(resp.IDToken)
	if err != nil {
		return nil, err
	}

	a.log.Info(claims)

	unique, err := apple.GetUniqueID(resp.IDToken)
	if err != nil {
		return nil, err
	}

	user, err := a.storage.GetOrCreateUser(ctx, unique)
	if err != nil {
		return nil, err
	}

	exp := time.Now().Add(time.Hour * JwtDurationHour).UTC()

	newClaims := &jwt.RegisteredClaims{
		Issuer:    user.String(),
		ExpiresAt: jwt.NewNumericDate(exp),
		ID:        uuid.NewV4().String(),
	}

	jwtToken := jwt.NewWithClaims(signingMethod, newClaims)

	block, _ := pem.Decode([]byte(a.cfg.Secret))
	if block == nil {
		return nil, errors.New("empty block after decoding")
	}

	privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	signedJWT, err := jwtToken.SignedString(privKey)
	if err != nil {
		return nil, err
	}

	session := &common.Session{
		JWT: signedJWT,
		User: &common.User{
			ID:      user,
			AppleID: unique,
		},
		ExpiredAt: exp,
	}

	return session, nil
}

func NewAuth(cfg *Configuration, storage storage, logger *logrus.Entry) Auth {
	return &auth{cfg: cfg, appleClient: apple.New(), storage: storage, log: logger}
}

func GetUser(ctx context.Context) (*common.User, error) {
	value := ctx.Value(userCtxKey)
	if user, ok := (value).(*common.User); ok {
		return user, nil
	}

	return nil, common.ErrUserNotFound
}
