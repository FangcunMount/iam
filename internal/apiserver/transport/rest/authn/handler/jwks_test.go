package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	jwksApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/jwks"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

type jwksPublisherStub struct {
	json []byte
	tag  jwksApp.CacheTag
	err  error
}

func (s *jwksPublisherStub) BuildJWKS(context.Context) ([]byte, jwksApp.CacheTag, error) {
	if s.err != nil {
		return nil, jwksApp.CacheTag{}, s.err
	}
	return s.json, s.tag, nil
}

func (s *jwksPublisherStub) GetPublishableKeys(context.Context) ([]*jwksApp.ManagedKey, error) {
	return nil, nil
}

func (s *jwksPublisherStub) ValidateCacheTag(context.Context, jwksApp.CacheTag) (bool, error) {
	return false, nil
}

func (s *jwksPublisherStub) GetCurrentCacheTag(context.Context) (jwksApp.CacheTag, error) {
	return s.tag, nil
}

func (s *jwksPublisherStub) RefreshCache(context.Context) error {
	return nil
}

func TestJWKSHandlerGetJWKSWritesCacheHeadersAndRawJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	lastModified := time.Date(2026, 4, 29, 10, 20, 30, 0, time.UTC)
	handler := newJWKSHandlerForTest(&jwksPublisherStub{
		json: []byte(`{"keys":[{"kid":"kid-1"}]}`),
		tag: jwksApp.CacheTag{
			ETag:         `"tag-1"`,
			LastModified: lastModified,
		},
	})

	w := performJWKSRequest(handler.GetJWKS, "")

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, `{"keys":[{"kid":"kid-1"}]}`, w.Body.String())
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	require.Equal(t, `"tag-1"`, w.Header().Get("ETag"))
	require.Equal(t, lastModified.Format(http.TimeFormat), w.Header().Get("Last-Modified"))
	require.Equal(t, "public, max-age=3600", w.Header().Get("Cache-Control"))
}

func TestJWKSHandlerGetJWKSReturnsNotModifiedWhenETagMatches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newJWKSHandlerForTest(&jwksPublisherStub{
		json: []byte(`{"keys":[]}`),
		tag: jwksApp.CacheTag{
			ETag:         `"tag-1"`,
			LastModified: time.Date(2026, 4, 29, 10, 20, 30, 0, time.UTC),
		},
	})

	w := performJWKSRequest(handler.GetJWKS, `"tag-1"`)

	require.Equal(t, http.StatusNotModified, w.Code)
	require.Empty(t, w.Body.String())
}

func TestJWKSHandlerGetJWKSReturnsErrorWhenBuildFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newJWKSHandlerForTest(&jwksPublisherStub{
		err: perrors.WithCode(code.ErrInternalServerError, "build jwks failed"),
	})

	w := performJWKSRequest(handler.GetJWKS, "")

	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func newJWKSHandlerForTest(publisher jwksApp.KeyPublisherPort) *JWKSHandler {
	return NewJWKSHandler(
		nil,
		jwksApp.NewKeyPublishAppService(publisher, log.NewLogger(zap.NewNop())),
	)
}

func performJWKSRequest(handler gin.HandlerFunc, ifNoneMatch string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	if ifNoneMatch != "" {
		c.Request.Header.Set("If-None-Match", ifNoneMatch)
	}

	handler(c)

	return w
}
