package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/Timothylock/go-signin-with-apple/apple"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/vektah/gqlparser/v2/gqlerror"

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
	Middleware() graphql.HandlerExtension
	WsInitFunc(ctx context.Context, payload transport.InitPayload) (context.Context, *transport.InitPayload, error)
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
			return nil, fmt.Errorf("Unexpected signing method: %v.", token.Header["alg"])
		}

		// hmacSampleSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
		key, err := a.getSecret()
		if err != nil {
			return nil, err
		}

		if privKey, ok := key.(*ecdsa.PrivateKey); ok {
			return &privKey.PublicKey, nil
		}

		return key, nil
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

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, err
	}

	return &common.User{
		// todo read from storage full user
		ID: userID,
		Session: common.Session{
			JWT:       jwtToken,
			ExpiredAt: exp.Time,
		},
	}, nil
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

	user, err := a.GetOrCreateUser(ctx, unique)
	if err != nil {
		return nil, err
	}

	exp := time.Now().Add(time.Hour * JwtDurationHour).UTC()

	newClaims := &jwt.RegisteredClaims{
		Issuer:    user.String(),
		ExpiresAt: jwt.NewNumericDate(exp),
		ID:        uuid.New().String(),
	}

	jwtToken := jwt.NewWithClaims(signingMethod, newClaims)

	privKey, err := a.getSecret()
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

func (a *auth) getSecret() (any, error) {
	block, _ := pem.Decode([]byte(a.cfg.Secret))
	if block == nil {
		return "", errors.New("empty block after decoding")
	}

	return x509.ParsePKCS8PrivateKey(block.Bytes)
}

func (a *auth) Middleware() graphql.HandlerExtension {
	return &Middleware{auth: a}
}

func NewAuth(cfg *Configuration, storage storage, logger *logrus.Entry) Auth {
	return &auth{cfg: cfg, appleClient: apple.New(), storage: storage, log: logger}
}

type Middleware struct {
	*auth
}

func (a *auth) WsInitFunc(ctx context.Context, payload transport.InitPayload) (context.Context, *transport.InitPayload, error) {
	authHeader := payload.Authorization()
	if authHeader == "" {
		return ctx, nil, nil
	}

	token := strings.Replace(authHeader, "Bearer ", "", 1)

	user, err := a.Validate(ctx, token)
	if err != nil {
		a.log.WithError(err).Warn("Invalid jwt")

		return ctx, nil, common.ErrJwtIncorrect
	}

	return context.WithValue(ctx, userCtxKey, user), nil, nil
}

func (a *Middleware) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	if !graphql.HasOperationContext(ctx) {
		return next(ctx)
	}

	rc := graphql.GetOperationContext(ctx)
	header := rc.Headers.Get("Authorization")
	// Allow unauthenticated users in
	if header == "" {
		return next(ctx)
	}

	token := strings.Replace(header, "Bearer ", "", 1)

	user, err := a.auth.Validate(ctx, token)
	if err != nil {
		a.log.WithError(err).Warn("Invalid jwt")
		// FIXME
		graphql.AddError(ctx, &gqlerror.Error{
			Message: common.ErrJwtIncorrect.Error(),
			Path:    graphql.GetPath(ctx),
			Extensions: map[string]interface{}{
				"code": "-1",
			},
		})

		return next(ctx)
	}

	// and call the next with our new context
	return next(context.WithValue(ctx, userCtxKey, user))
}

func (a *Middleware) ExtensionName() string {
	return "Auth"
}

func (a *Middleware) Validate(graphql.ExecutableSchema) error {
	return nil
}

func GetUser(ctx context.Context) (*common.User, error) {
	value := ctx.Value(userCtxKey)
	if user, ok := (value).(*common.User); ok {
		return user, nil
	}

	return nil, common.ErrUserNotFound
}
