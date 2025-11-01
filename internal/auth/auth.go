// Package auth provides JWT and Apple Sign In authentication, along with GraphQL middleware.
//
//nolint:wsl_v5 // allow compact style; this file frequently returns early and uses short guard clauses
package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
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

// ErrAppleAuth indicates that Apple Sign In authentication failed.
var (
	ErrAppleAuth                  = errors.New("apple authentication failed")
	ErrEmptyBlockDecode           = errors.New("empty block after decoding")
	ErrAdminKeyNotECDSA           = errors.New("admin public key is not ECDSA")
	ErrAdminVerificationKeyAbsent = errors.New("admin verification key not loaded")
)

type ctxKey string

const (
	userCtxKey ctxKey = "user"
	// JwtDurationHour is the number of hours for which the issued JWT will be valid.
	JwtDurationHour = 24
)

var signingMethod = jwt.SigningMethodES256

var errUnexpectedSigningMethod = errors.New("unexpected signing method")

const (
	bearerPrefix             = "Bearer "
	invalidJWTMsg            = "Invalid jwt"
	getSecretWrapF           = "get secret: %w"
	unexpectedSigningMethodF = "%w: %v"
	tokenPrefixLogLen        = 20
)

// Admin auth constants
const (
	AdminClaimKey    = "admin"
	AdminIssuer      = "TeaElephantEditor"
	AdminAudience    = "tea-elephant-api"
	ClockSkewSeconds = 300
)

// AdminPrincipal represents an authenticated admin session
type AdminPrincipal struct {
	JTI       string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// context key for admin principal
const adminCtxKey ctxKey = "adminPrincipal"

// Auth defines the authentication operations for issuing and validating JWTs
// and providing GraphQL middleware support.
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

	// cached admin public keys by kid (empty kid = default)
	adminKeys      map[string]*ecdsa.PublicKey
	adminKeysMutex sync.RWMutex

	storage
	log *logrus.Entry
}

func (a *auth) Validate(_ context.Context, jwtToken string) (*common.User, error) {
	result, err := jwt.Parse(jwtToken, a.verificationKey)
	if err != nil {
		return nil, fmt.Errorf("parse jwt: %w", err)
	}

	claims, exp, err := extractValidClaims(result)
	if err != nil {
		return nil, err
	}

	userID, err := issuerUUID(claims)
	if err != nil {
		return nil, err
	}

	return &common.User{
		// todo read from storage full user
		ID: userID,
		Session: common.Session{
			JWT:       jwtToken,
			ExpiredAt: exp,
		},
	}, nil
}

// verificationKey validates the signing method and returns the appropriate key for verification
func (a *auth) verificationKey(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
		return nil, fmt.Errorf(unexpectedSigningMethodF, errUnexpectedSigningMethod, token.Header["alg"])
	}

	key, err := a.getSecret()
	if err != nil {
		return nil, fmt.Errorf(getSecretWrapF, err)
	}

	if privKey, ok := key.(*ecdsa.PrivateKey); ok {
		return &privKey.PublicKey, nil
	}

	return key, nil
}

func extractValidClaims(result *jwt.Token) (jwt.MapClaims, time.Time, error) {
	claims, ok := result.Claims.(jwt.MapClaims)
	if !ok || !result.Valid {
		return nil, time.Time{}, common.ErrInvalidToken
	}

	exp, err := claims.GetExpirationTime()
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("get expiration time: %w", err)
	}

	if time.Now().After(exp.Time) {
		return nil, time.Time{}, common.ErrExpiredToken
	}

	return claims, exp.Time, nil
}

func issuerUUID(claims jwt.MapClaims) (uuid.UUID, error) {
	userIDStr, err := claims.GetIssuer()
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("get issuer: %w", err)
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("parse uuid: %w", err)
	}

	return userID, nil
}

func (a *auth) Start() (err error) {
	a.secret, err = apple.GenerateClientSecret(a.cfg.Secret, a.cfg.TeamID, a.cfg.ClientID, a.cfg.KeyID)
	if err != nil {
		return fmt.Errorf("generate apple client secret: %w", err)
	}

	// preload admin public key(s)
	if err := a.loadAdminKeys(); err != nil {
		return fmt.Errorf("load admin keys: %w", err)
	}

	return nil
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
		return nil, fmt.Errorf("verify apple app token: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("%w: %s", ErrAppleAuth, resp.ErrorDescription)
	}

	claims, err := apple.GetClaims(resp.IDToken)
	if err != nil {
		return nil, fmt.Errorf("get claims: %w", err)
	}

	a.log.Info(claims)

	unique, err := apple.GetUniqueID(resp.IDToken)
	if err != nil {
		return nil, fmt.Errorf("get unique id: %w", err)
	}

	user, err := a.GetOrCreateUser(ctx, unique)
	if err != nil {
		return nil, fmt.Errorf("get or create user: %w", err)
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
		return nil, fmt.Errorf(getSecretWrapF, err)
	}

	signedJWT, err := jwtToken.SignedString(privKey)
	if err != nil {
		return nil, fmt.Errorf("sign jwt: %w", err)
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
		return "", ErrEmptyBlockDecode
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse pkcs8 private key: %w", err)
	}

	return key, nil
}

// Middleware returns the GraphQL handler extension that enforces authentication.
func (a *auth) Middleware() graphql.HandlerExtension {
	return &Middleware{auth: a}
}

// NewAuth constructs the Auth service with provided configuration, storage, and logger.
func NewAuth(cfg *Configuration, storage storage, logger *logrus.Entry) Auth {
	return &auth{cfg: cfg, appleClient: apple.New(), storage: storage, log: logger}
}

// Middleware implements a GraphQL extension to authenticate requests.
type Middleware struct {
	*auth
}

// WsInitFunc initializes the WebSocket connection by validating Authorization header if provided.
func (a *auth) WsInitFunc(ctx context.Context, payload transport.InitPayload) (context.Context, *transport.InitPayload, error) {
	authHeader := payload.Authorization()
	if authHeader == "" {
		return ctx, nil, nil
	}

	token := strings.Replace(authHeader, bearerPrefix, "", 1)

	// Try user token first
	if user, err := a.Validate(ctx, token); err == nil {
		ctx = context.WithValue(ctx, userCtxKey, user)
	} else {
		// Try admin token
		if principal, aerr := a.ValidateAdmin(ctx, token); aerr == nil {
			ctx = context.WithValue(ctx, adminCtxKey, principal)
		} else {
			a.log.WithError(err).WithField("admin_err", aerr).Warn(invalidJWTMsg)
			return ctx, nil, common.ErrJwtIncorrect
		}
	}

	return ctx, nil, nil
}

// InterceptResponse intercepts GraphQL responses to ensure the user is authenticated.
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

	token := strings.Replace(header, bearerPrefix, "", 1)

	// Try as user token
	if user, err := a.auth.Validate(ctx, token); err == nil {
		return next(context.WithValue(ctx, userCtxKey, user))
	}
	// Try as admin token
	if principal, err := a.ValidateAdmin(ctx, token); err == nil {
		return next(context.WithValue(ctx, adminCtxKey, principal))
	}

	// Neither user nor admin -> add GraphQL error with stable code
	graphql.AddError(ctx, &gqlerror.Error{
		Message: common.ErrJwtIncorrect.Error(),
		Path:    graphql.GetPath(ctx),
		Extensions: map[string]interface{}{
			"code": "UNAUTHENTICATED",
		},
	})
	return next(ctx)
}

// ExtensionName returns the name of the GraphQL extension.
func (a *Middleware) ExtensionName() string {
	return "Auth"
}

// Validate implements the GraphQL extension validator (no-op).
func (a *Middleware) Validate(graphql.ExecutableSchema) error {
	return nil
}

// GetUser extracts the authenticated user from the context.
func GetUser(ctx context.Context) (*common.User, error) {
	value := ctx.Value(userCtxKey)
	if user, ok := (value).(*common.User); ok {
		return user, nil
	}

	return nil, common.ErrUserNotFound
}

// AdminPrincipalFrom extracts the admin principal from context.
func AdminPrincipalFrom(ctx context.Context) (*AdminPrincipal, bool) {
	v := ctx.Value(adminCtxKey)
	p, ok := v.(*AdminPrincipal)
	return p, ok
}

// RequireAdmin ensures an admin principal is present in context.
func RequireAdmin(ctx context.Context) error {
	if _, ok := AdminPrincipalFrom(ctx); !ok {
		return common.ErrUnauthorized
	}
	return nil
}

// loadPublicKeyFromFile reads an ECDSA public key from a PEM file.
func (a *auth) loadPublicKeyFromFile(path string) (*ecdsa.PublicKey, error) {
	// #nosec G304 -- path comes from trusted configuration (mounted secret)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read admin public key: %w", err)
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, ErrEmptyBlockDecode
	}
	pk, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse admin public key: %w", err)
	}
	ecdsaKey, ok := pk.(*ecdsa.PublicKey)
	if !ok {
		return nil, ErrAdminKeyNotECDSA
	}
	return ecdsaKey, nil
}

// loadAdminKeys preloads the default admin public key into cache.
func (a *auth) loadAdminKeys() error {
	a.adminKeysMutex.Lock()
	defer a.adminKeysMutex.Unlock()
	if a.adminKeys == nil {
		a.adminKeys = make(map[string]*ecdsa.PublicKey)
	}
	key, err := a.loadPublicKeyFromFile(a.cfg.AdminPublicKeyPath)
	if err != nil {
		return err
	}
	// empty kid denotes default key
	a.adminKeys[""] = key
	return nil
}

// adminVerificationKey selects the correct admin public key from cache.
func (a *auth) adminVerificationKey(token *jwt.Token) (interface{}, error) {
	// Enforce ES256 algorithm
	if token.Method.Alg() != signingMethod.Alg() {
		a.log.WithField("alg", token.Header["alg"]).Warn("Unexpected signing method for admin JWT")
		return nil, fmt.Errorf(unexpectedSigningMethodF, errUnexpectedSigningMethod, token.Header["alg"])
	}

	kid, _ := token.Header["kid"].(string)
	a.log.WithField("kid", kid).Debug("Looking up admin public key")

	a.adminKeysMutex.RLock()
	key, ok := a.adminKeys[kid]
	if !ok {
		// fall back to default if no kid provided
		key, ok = a.adminKeys[""]
		a.log.Debug("Using default admin key (no kid match)")
	}
	a.adminKeysMutex.RUnlock()

	if !ok || key == nil {
		a.log.Warn("Admin verification key not found in cache")
		return nil, ErrAdminVerificationKeyAbsent
	}

	return key, nil
}

// ValidateAdmin parses and validates an admin JWT and returns a principal.
func (a *auth) ValidateAdmin(_ context.Context, jwtToken string) (*AdminPrincipal, error) {
	a.log.WithField("token_prefix", jwtToken[:min(tokenPrefixLogLen, len(jwtToken))]).Debug("ValidateAdmin called")

	parsed, err := jwt.Parse(jwtToken, a.adminVerificationKey,
		jwt.WithValidMethods([]string{signingMethod.Alg()}),
		jwt.WithIssuer(AdminIssuer),
		jwt.WithAudience(AdminAudience),
		jwt.WithLeeway(ClockSkewSeconds*time.Second),
	)
	if err != nil {
		a.log.WithError(err).Warn("Admin JWT parse failed")
		return nil, fmt.Errorf("parse admin jwt: %w", err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		a.log.Warn("Admin JWT invalid or claims extraction failed")
		return nil, common.ErrInvalidToken
	}

	// Check custom admin claim
	isAdmin, _ := claims[AdminClaimKey].(bool)
	if !isAdmin {
		a.log.WithField("admin_claim", claims[AdminClaimKey]).Warn("Admin claim missing or false")
		return nil, common.ErrNotAdmin
	}

	a.log.WithFields(map[string]interface{}{
		"jti": claims["jti"],
		"iss": claims["iss"],
		"aud": claims["aud"],
	}).Debug("Admin JWT validated successfully")
	// Build principal
	var jti string
	if v, ok := claims["jti"].(string); ok {
		jti = v
	}
	issuedAt, err := claims.GetIssuedAt()
	if err != nil {
		return nil, fmt.Errorf("missing iat: %w", err)
	}
	exp, err := claims.GetExpirationTime()
	if err != nil {
		return nil, fmt.Errorf("missing exp: %w", err)
	}
	return &AdminPrincipal{
		JTI:       jti,
		IssuedAt:  issuedAt.Time,
		ExpiresAt: exp.Time,
	}, nil
}
