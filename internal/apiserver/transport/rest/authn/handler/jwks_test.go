package handler

import (
	"bytes"
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

	jwksApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/jwks"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

type jwksPublisherStub struct {
	json           []byte
	tag            jwksApp.CacheTag
	err            error
	publishableErr error
}

func (s *jwksPublisherStub) BuildJWKS(context.Context) ([]byte, jwksApp.CacheTag, error) {
	if s.err != nil {
		return nil, jwksApp.CacheTag{}, s.err
	}
	return s.json, s.tag, nil
}

func (s *jwksPublisherStub) GetPublishableKeys(context.Context) ([]*jwksApp.ManagedKey, error) {
	if s.publishableErr != nil {
		return nil, s.publishableErr
	}
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

func TestJWKSHandlerCreateKeyReturnsBindError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newJWKSAdminHandlerForTest(&jwksKeyManagerStub{}, &jwksPublisherStub{})

	w := performJWKSAdminRequest(handler.CreateKey, http.MethodPost, "/admin/jwks/keys", []byte(`{`), nil)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestJWKSHandlerListKeysRejectsInvalidQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newJWKSAdminHandlerForTest(&jwksKeyManagerStub{}, &jwksPublisherStub{})

	w := performJWKSAdminRequest(handler.ListKeys, http.MethodGet, "/admin/jwks/keys?status=invalid", nil, nil)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestJWKSHandlerListKeysPropagatesApplicationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newJWKSAdminHandlerForTest(&jwksKeyManagerStub{
		listErr: perrors.WithCode(code.ErrInternalServerError, "list failed"),
	}, &jwksPublisherStub{})

	w := performJWKSAdminRequest(handler.ListKeys, http.MethodGet, "/admin/jwks/keys", nil, nil)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestJWKSHandlerGetKeyRequiresKid(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newJWKSAdminHandlerForTest(&jwksKeyManagerStub{}, &jwksPublisherStub{})

	w := performJWKSAdminRequest(handler.GetKey, http.MethodGet, "/admin/jwks/keys/", nil, nil)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestJWKSHandlerGetKeyPropagatesApplicationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newJWKSAdminHandlerForTest(&jwksKeyManagerStub{
		getErr: perrors.WithCode(code.ErrKeyNotFound, "key not found"),
	}, &jwksPublisherStub{})

	w := performJWKSAdminRequest(handler.GetKey, http.MethodGet, "/admin/jwks/keys/kid-1", nil, gin.Params{
		{Key: "kid", Value: "kid-1"},
	})

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestJWKSHandlerRetireKeyPropagatesApplicationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newJWKSAdminHandlerForTest(&jwksKeyManagerStub{
		retireErr: perrors.WithCode(code.ErrInvalidArgument, "invalid transition"),
	}, &jwksPublisherStub{})

	w := performJWKSAdminRequest(handler.RetireKey, http.MethodPost, "/admin/jwks/keys/kid-1/retire", nil, gin.Params{
		{Key: "kid", Value: "kid-1"},
	})

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestJWKSHandlerForceRetireKeyPropagatesApplicationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newJWKSAdminHandlerForTest(&jwksKeyManagerStub{
		forceRetireErr: perrors.WithCode(code.ErrKeyNotFound, "key not found"),
	}, &jwksPublisherStub{})

	w := performJWKSAdminRequest(handler.ForceRetireKey, http.MethodPost, "/admin/jwks/keys/kid-1/force-retire", nil, gin.Params{
		{Key: "kid", Value: "kid-1"},
	})

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestJWKSHandlerEnterGracePeriodPropagatesApplicationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newJWKSAdminHandlerForTest(&jwksKeyManagerStub{
		graceErr: perrors.WithCode(code.ErrInvalidArgument, "invalid transition"),
	}, &jwksPublisherStub{})

	w := performJWKSAdminRequest(handler.EnterGracePeriod, http.MethodPost, "/admin/jwks/keys/kid-1/grace", nil, gin.Params{
		{Key: "kid", Value: "kid-1"},
	})

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestJWKSHandlerCleanupExpiredKeysPropagatesApplicationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newJWKSAdminHandlerForTest(&jwksKeyManagerStub{
		cleanupErr: perrors.WithCode(code.ErrInternalServerError, "cleanup failed"),
	}, &jwksPublisherStub{})

	w := performJWKSAdminRequest(handler.CleanupExpiredKeys, http.MethodPost, "/admin/jwks/keys/cleanup", nil, nil)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestJWKSHandlerGetPublishableKeysPropagatesApplicationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newJWKSAdminHandlerForTest(&jwksKeyManagerStub{}, &jwksPublisherStub{
		publishableErr: perrors.WithCode(code.ErrInternalServerError, "publisher failed"),
	})

	w := performJWKSAdminRequest(handler.GetPublishableKeys, http.MethodGet, "/admin/jwks/keys/publishable", nil, nil)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func newJWKSHandlerForTest(publisher jwksApp.KeyPublisherPort) *JWKSHandler {
	return NewJWKSHandler(
		nil,
		jwksApp.NewKeyPublishAppService(publisher, log.NewLogger(zap.NewNop())),
	)
}

func newJWKSAdminHandlerForTest(manager jwksApp.KeyManagerPort, publisher jwksApp.KeyPublisherPort) *JWKSHandler {
	return NewJWKSHandler(
		jwksApp.NewKeyManagementAppService(manager, log.NewLogger(zap.NewNop())),
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

func performJWKSAdminRequest(handler gin.HandlerFunc, method, path string, body []byte, params gin.Params) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = params
	c.Request = httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		c.Request.Header.Set("Content-Type", "application/json")
	}

	handler(c)

	return w
}

type jwksKeyManagerStub struct {
	createErr      error
	getErr         error
	retireErr      error
	forceRetireErr error
	graceErr       error
	cleanupErr     error
	listErr        error
}

func (s *jwksKeyManagerStub) CreateKey(context.Context, string, *time.Time, *time.Time) (*jwksApp.ManagedKey, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	return nil, nil
}

func (s *jwksKeyManagerStub) GetActiveKey(context.Context) (*jwksApp.ManagedKey, error) {
	return nil, nil
}

func (s *jwksKeyManagerStub) GetKeyByKid(context.Context, string) (*jwksApp.ManagedKey, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return nil, nil
}

func (s *jwksKeyManagerStub) RetireKey(context.Context, string) error {
	return s.retireErr
}

func (s *jwksKeyManagerStub) ForceRetireKey(context.Context, string) error {
	return s.forceRetireErr
}

func (s *jwksKeyManagerStub) EnterGracePeriod(context.Context, string) error {
	return s.graceErr
}

func (s *jwksKeyManagerStub) CleanupExpiredKeys(context.Context) (int, error) {
	if s.cleanupErr != nil {
		return 0, s.cleanupErr
	}
	return 0, nil
}

func (s *jwksKeyManagerStub) ListKeys(context.Context, jwksApp.KeyStatus, int, int) ([]*jwksApp.ManagedKey, int64, error) {
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	return nil, 0, nil
}
