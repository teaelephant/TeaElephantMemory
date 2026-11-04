package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/teaelephant/TeaElephantMemory/common"
)

const (
	fixtureImagePath    = "/v2/tea-images/11111111-1111-1111-1111-111111111111"
	fixtureContentType  = "image/png"
	fixtureImageBytes   = "PNG bytes"
	contentTypeHeader   = "Content-Type"
	nosniffHeader       = "X-Content-Type-Options"
	contentLengthHeader = "Content-Length"
	etagHeader          = "ETag"
)

func TestInvalidTeaImagePathReturnsNotFound(t *testing.T) {
	t.Parallel()

	s := NewServer(nil, nil, nil)
	s.InitV2Api()

	for _, path := range []string{"/v2/tea-images/invalid", fixtureImagePath} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		s.router.ServeHTTP(response, request)
		require.Equal(t, http.StatusNotFound, response.Code)
	}
}

type imageFixture struct {
	content *common.TeaImageContent
	err     error
}

func (f imageFixture) ReadTeaImage(context.Context, uuid.UUID) (*common.TeaImageContent, error) {
	return f.content, f.err
}

func TestTeaImageHTTPReturnsBytesAndMatchingHeadHeaders(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	content := &common.TeaImageContent{
		TeaImage: common.TeaImage{ID: id, ContentType: fixtureContentType, CreatedAt: time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)},
		Data:     []byte(fixtureImageBytes),
	}
	s := NewServer(nil, nil, nil, imageFixture{content: content})
	s.InitV2Api()

	path := "/v2/tea-images/" + id.String()

	get := httptest.NewRecorder()
	s.router.ServeHTTP(get, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, get.Code)
	require.Equal(t, []byte(fixtureImageBytes), get.Body.Bytes())
	require.Equal(t, fixtureContentType, get.Header().Get(contentTypeHeader))
	require.Equal(t, "nosniff", get.Header().Get(nosniffHeader))
	require.Equal(t, "9", get.Header().Get(contentLengthHeader))

	head := httptest.NewRecorder()
	s.router.ServeHTTP(head, httptest.NewRequestWithContext(t.Context(), http.MethodHead, path, nil))
	require.Equal(t, http.StatusOK, head.Code)
	require.Empty(t, head.Body.Bytes())

	for _, header := range []string{contentTypeHeader, contentLengthHeader, nosniffHeader, etagHeader, "Last-Modified", "Cache-Control"} {
		require.Equal(t, get.Header().Get(header), head.Header().Get(header), header)
	}

	cachedRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	cachedRequest.Header.Set("If-None-Match", get.Header().Get(etagHeader))

	cached := httptest.NewRecorder()
	s.router.ServeHTTP(cached, cachedRequest)
	require.Equal(t, http.StatusNotModified, cached.Code)
	require.Empty(t, cached.Body.Bytes())
}

func TestTeaImageHTTPMissingImageReturnsNotFound(t *testing.T) {
	t.Parallel()

	s := NewServer(nil, nil, nil, imageFixture{err: common.ErrTeaImageNotFound})
	s.InitV2Api()

	response := httptest.NewRecorder()
	s.router.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, fixtureImagePath, nil))
	require.Equal(t, http.StatusNotFound, response.Code)
}
