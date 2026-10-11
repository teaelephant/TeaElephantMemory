package graphql

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/teaelephant/TeaElephantMemory/common"
	authPkg "github.com/teaelephant/TeaElephantMemory/internal/auth"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/qr"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/tea"
	"github.com/teaelephant/TeaElephantMemory/internal/teaimages"
	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/graphql/generated"
	pgadapter "github.com/teaelephant/TeaElephantMemory/pkg/pg"
)

const (
	testJSONContentType   = "application/json"
	testTeaVariable       = "tea"
	testPGDriver          = "pgx"
	testGraphQLQueryKey   = "query"
	testGraphQLPath       = "/v2/query"
	testContentTypeHeader = "Content-Type"
	testImageVariable     = "image"
	testAddImageField     = "addTeaImage"
	testRenamedTea        = "Renamed tea"
)

// Integration tests use only an explicitly supplied disposable PostgreSQL server.
func imageTestDatabase(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEA_IMAGES_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("TEA_IMAGES_TEST_PG_DSN is not set; PostgreSQL integration test skipped")
	}

	admin, err := sql.Open(testPGDriver, dsn)
	require.NoError(t, err)

	schema := "tea_images_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.ExecContext(t.Context(), `CREATE SCHEMA `+schema)
	require.NoError(t, err)

	parsed, err := url.Parse(dsn)
	require.NoError(t, err)

	values := parsed.Query()
	values.Set("search_path", schema)
	parsed.RawQuery = values.Encode()
	db, err := sql.Open(testPGDriver, parsed.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())

		_, err := admin.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		require.NoError(t, err)
		require.NoError(t, admin.Close())
	})

	source, err := os.ReadFile("../../../../db/schema.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), string(source))
	require.NoError(t, err)
	// Exercise an existing database that predates the image table.
	_, err = db.ExecContext(t.Context(), `DROP TABLE tea_images`)
	require.NoError(t, err)

	migration, err := os.ReadFile("../../../../db/migrations/20261011_tea_images.sql")
	require.NoError(t, err)

	for range 2 {
		_, err = db.ExecContext(t.Context(), string(migration))
		require.NoError(t, err, "existing-database migration must be idempotent")
	}

	return db
}

func imageTestAuth(t *testing.T, store interface {
	GetOrCreateUser(ctx context.Context, appleID string) (uuid.UUID, error)
}) (authPkg.Auth, string, string) {
	t.Helper()

	userID, err := store.GetOrCreateUser(t.Context(), "test-user")
	require.NoError(t, err)
	auth, tokens, adminToken := imageTestAuthUsers(t, store, userID)

	return auth, tokens[0], adminToken
}

func imageTestAuthUsers(t *testing.T, store interface {
	GetOrCreateUser(ctx context.Context, appleID string) (uuid.UUID, error)
}, userIDs ...uuid.UUID) (authPkg.Auth, []string, string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	privateKey, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	publicKey, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	publicPath := filepath.Join(t.TempDir(), "public.pem")
	require.NoError(t, os.WriteFile(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKey}), 0o600))
	cfg := &authPkg.Configuration{
		Secret: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKey})),
		TeamID: "test-team", ClientID: "test-app", KeyID: "test-key", AdminPublicKeyPath: publicPath,
	}
	auth := authPkg.NewAuth(cfg, store, logrus.NewEntry(logrus.New()))
	require.NoError(t, auth.Start())

	userTokens := make([]string, len(userIDs))
	for index, userID := range userIDs {
		userTokens[index], err = jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{
			Issuer: userID.String(), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}).SignedString(key)
		require.NoError(t, err)
	}

	adminJWT, err := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": authPkg.AdminIssuer, "aud": authPkg.AdminAudience, "admin": true,
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(), "jti": uuid.NewString(),
	}).SignedString(key)
	require.NoError(t, err)

	return auth, userTokens, adminJWT
}

type imageGraphQLReply struct {
	Data   map[string]json.RawMessage `json:"data"`
	Errors []struct {
		Message    string         `json:"message"`
		Extensions map[string]any `json:"extensions"`
	} `json:"errors"`
}

func imageGraphQLCall(t *testing.T, server http.Handler, token, query string, variables map[string]any) imageGraphQLReply {
	t.Helper()

	body, err := json.Marshal(map[string]any{testGraphQLQueryKey: query, "variables": variables})
	require.NoError(t, err)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, testGraphQLPath, bytes.NewReader(body))
	request.Header.Set(testContentTypeHeader, testJSONContentType)

	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	var result imageGraphQLReply
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))

	return result
}

const imageSelection = `id name images { id url contentType width height createdAt isCover }`

//nolint:tagliatelle // Response fields use the GraphQL schema wire names.
type teaImageReply struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Images []struct {
		ID          string    `json:"id"`
		URL         string    `json:"url"`
		ContentType string    `json:"contentType"`
		Width       int       `json:"width"`
		Height      int       `json:"height"`
		CreatedAt   time.Time `json:"createdAt"`
		IsCover     bool      `json:"isCover"`
	} `json:"images"`
}

func decodeTeaImageReply(t *testing.T, reply imageGraphQLReply, field string) teaImageReply {
	t.Helper()

	require.Empty(t, reply.Errors)

	var result teaImageReply
	require.NoError(t, json.Unmarshal(reply.Data[field], &result))

	return result
}

func imageFixtureBase64(t *testing.T) string {
	t.Helper()

	var data bytes.Buffer
	require.NoError(t, png.Encode(&data, image.NewGray(image.Rect(0, 0, 2, 3))))

	return base64.StdEncoding.EncodeToString(data.Bytes())
}

//nolint:funlen // Exercise one complete public catalog lifecycle against the real database and authentication middleware.
func TestTeaImagesGraphQLSharedCatalogueLifecycle(t *testing.T) {
	db := imageTestDatabase(t)
	store := pgadapter.NewDB(db, logrus.NewEntry(logrus.New()))
	manager := tea.NewManager(store)
	images := teaimages.NewService(store)
	auth, userToken, adminToken := imageTestAuth(t, store)
	resolver := NewResolver(nil, manager, qr.NewManager(store), nil, nil, nil, nil, nil, nil, nil, nil, nil, images)
	server := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	server.Use(auth.Middleware())

	record, err := store.WriteRecord(t.Context(), &common.TeaData{Name: testSharedTeaName, Type: common.TeaBeverageType, Description: "Original metadata"})
	require.NoError(t, err)
	otherTea, err := store.WriteRecord(t.Context(), &common.TeaData{Name: "Another tea", Type: common.TeaBeverageType})
	require.NoError(t, err)
	encoded := imageFixtureBase64(t)
	addQuery := `mutation($tea:ID!,$image:String!) { addTeaImage(teaID:$tea,imageBase64:$image) { ` + imageSelection + ` } }`
	imageQuery := `mutation($tea:ID!,$image:ID!) { setTeaCoverImage(teaID:$tea,imageID:$image) { ` + imageSelection + ` } }`
	deleteQuery := `mutation($tea:ID!,$image:ID!) { deleteTeaImage(teaID:$tea,imageID:$image) { ` + imageSelection + ` } }`
	vars := map[string]any{testTeaVariable: record.ID.String(), testImageVariable: encoded}

	first := decodeTeaImageReply(t, imageGraphQLCall(t, server, userToken, addQuery, vars), testAddImageField)
	require.Len(t, first.Images, 1)
	require.True(t, first.Images[0].IsCover)
	require.Equal(t, "image/png", first.Images[0].ContentType)
	require.Equal(t, 2, first.Images[0].Width)
	require.Equal(t, 3, first.Images[0].Height)
	require.False(t, first.Images[0].CreatedAt.IsZero())
	require.Equal(t, "/v2/tea-images/"+first.Images[0].ID, first.Images[0].URL)
	second := decodeTeaImageReply(t, imageGraphQLCall(t, server, adminToken, addQuery, vars), testAddImageField)
	require.Len(t, second.Images, 2)
	require.True(t, second.Images[0].IsCover)
	require.False(t, second.Images[1].IsCover)

	for range 2 {
		qrID := uuid.New()
		require.NoError(t, store.WriteQR(t.Context(), qrID, &common.QR{Tea: record.ID, BowlingTemp: 90, ExpirationDate: time.Now().Add(time.Hour)}))
		response := imageGraphQLCall(t, server, "", `query($id:ID!) { qrRecord(id:$id) { tea { `+imageSelection+` } } }`, map[string]any{"id": qrID.String()})
		require.Empty(t, response.Errors)

		var result struct {
			Tea teaImageReply `json:"tea"`
		}
		require.NoError(t, json.Unmarshal(response.Data["qrRecord"], &result))
		require.Equal(t, second.Images, result.Tea.Images)
	}

	wrong := map[string]any{testTeaVariable: otherTea.ID.String(), testImageVariable: first.Images[0].ID}
	for _, query := range []string{imageQuery, deleteQuery} {
		reply := imageGraphQLCall(t, server, adminToken, query, wrong)
		require.Len(t, reply.Errors, 1)
		require.Equal(t, roleNotFoundCode, reply.Errors[0].Extensions["code"])
	}

	coverVars := map[string]any{testTeaVariable: record.ID.String(), testImageVariable: second.Images[1].ID}
	changed := decodeTeaImageReply(t, imageGraphQLCall(t, server, userToken, imageQuery, coverVars), "setTeaCoverImage")
	require.False(t, changed.Images[0].IsCover)
	require.True(t, changed.Images[1].IsCover)

	for _, token := range []string{"", userToken} {
		denialCode := testImageForbiddenCode
		if token == "" {
			denialCode = testImageUnauthorizedCode
		}

		denied := imageGraphQLCall(t, server, token, deleteQuery, coverVars)
		require.Len(t, denied.Errors, 1)
		require.Equal(t, denialCode, denied.Errors[0].Extensions["code"])
		require.Empty(t, denied.Data)
		persisted := decodeTeaImageReply(t, imageGraphQLCall(t, server, "", `query($tea:ID!) { tea(id:$tea) { `+imageSelection+` } }`, map[string]any{testTeaVariable: record.ID.String()}), testTeaVariable)
		require.Equal(t, changed.Images, persisted.Images, "denied deletion must preserve the gallery and cover")
	}

	remaining := decodeTeaImageReply(t, imageGraphQLCall(t, server, adminToken, deleteQuery, coverVars), testDeleteImageField)
	require.Len(t, remaining.Images, 1)
	require.Equal(t, first.Images[0].ID, remaining.Images[0].ID)
	require.True(t, remaining.Images[0].IsCover)

	_, err = store.Update(t.Context(), record.ID, &common.TeaData{Name: testRenamedTea, Type: common.TeaBeverageType, Description: "New metadata"})
	require.NoError(t, err)
	updated := decodeTeaImageReply(t, imageGraphQLCall(t, server, "", `query($tea:ID!) { tea(id:$tea) { `+imageSelection+` } }`, map[string]any{testTeaVariable: record.ID.String()}), testTeaVariable)
	require.Equal(t, remaining.Images, updated.Images)
	require.Equal(t, testRenamedTea, updated.Name)
	invalid := imageGraphQLCall(t, server, userToken, addQuery, map[string]any{testTeaVariable: record.ID.String(), testImageVariable: "AA=="})
	require.Len(t, invalid.Errors, 1)
	require.Equal(t, "BAD_USER_INPUT", invalid.Errors[0].Extensions["code"])
	require.NoError(t, store.Delete(t.Context(), record.ID))
	_, err = store.ReadTeaImage(t.Context(), uuid.MustParse(first.Images[0].ID))
	require.ErrorIs(t, err, common.ErrTeaImageNotFound)
}

func TestTeaImagesConcurrentLimitAndSingleCover(t *testing.T) {
	db := imageTestDatabase(t)
	store := pgadapter.NewDB(db, logrus.NewEntry(logrus.New()))
	images := teaimages.NewService(store)
	record, err := store.WriteRecord(t.Context(), &common.TeaData{Name: "Concurrent tea", Type: common.TeaBeverageType})
	require.NoError(t, err)
	encoded := imageFixtureBase64(t)

	var writes sync.WaitGroup

	results := make(chan error, 20)

	for range 20 {
		writes.Go(func() { results <- images.Add(t.Context(), record.ID, encoded) })
	}

	writes.Wait()
	close(results)

	successful := 0

	for err := range results {
		if err == nil {
			successful++
		} else {
			require.ErrorIs(t, err, common.ErrTeaImageLimit)
		}
	}

	require.Equal(t, 10, successful)
	metadata, err := images.ListTeaImages(t.Context(), record.ID)
	require.NoError(t, err)
	require.Len(t, metadata, 10)

	coverResults := make(chan error, len(metadata))
	for _, item := range metadata {
		writes.Go(func() { coverResults <- images.SetTeaCoverImage(t.Context(), record.ID, item.ID) })
	}

	writes.Wait()
	close(coverResults)

	for err := range coverResults {
		require.NoError(t, err)
	}

	metadata, err = images.ListTeaImages(t.Context(), record.ID)
	require.NoError(t, err)

	covers := 0

	for _, item := range metadata {
		if item.IsCover {
			covers++
		}
	}

	require.Equal(t, 1, covers)
	require.ErrorIs(t, images.Add(t.Context(), uuid.New(), encoded), common.ErrTeaNotFound)
}
