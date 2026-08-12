package idp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	appsvc "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/wechatapp"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/idp/handler"
	idpresponse "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/idp/response"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/pkg/core"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fakeWechatAppService struct {
	listResult       []*appsvc.WechatAppResult
	updateResult     *appsvc.WechatAppResult
	enableResult     *appsvc.WechatAppResult
	disableResult    *appsvc.WechatAppResult
	lastListFilter   appsvc.ListWechatAppsFilter
	lastUpdateAppID  string
	lastUpdateDTO    appsvc.UpdateWechatAppDTO
	lastEnableAppID  string
	lastDisableAppID string
}

func (f *fakeWechatAppService) CreateApp(context.Context, appsvc.CreateWechatAppDTO) (*appsvc.WechatAppResult, error) {
	return nil, nil
}

func (f *fakeWechatAppService) GetApp(context.Context, string) (*appsvc.WechatAppResult, error) {
	return nil, nil
}

func (f *fakeWechatAppService) ListApps(_ context.Context, filter appsvc.ListWechatAppsFilter) ([]*appsvc.WechatAppResult, error) {
	f.lastListFilter = filter
	return f.listResult, nil
}

func (f *fakeWechatAppService) UpdateApp(_ context.Context, appID string, dto appsvc.UpdateWechatAppDTO) (*appsvc.WechatAppResult, error) {
	f.lastUpdateAppID = appID
	f.lastUpdateDTO = dto
	return f.updateResult, nil
}

func (f *fakeWechatAppService) EnableApp(_ context.Context, appID string) (*appsvc.WechatAppResult, error) {
	f.lastEnableAppID = appID
	return f.enableResult, nil
}

func (f *fakeWechatAppService) DisableApp(_ context.Context, appID string) (*appsvc.WechatAppResult, error) {
	f.lastDisableAppID = appID
	return f.disableResult, nil
}

type fakeCredentialService struct{}

func (fakeCredentialService) RotateAuthSecret(context.Context, string, string) error { return nil }
func (fakeCredentialService) RotateMsgSecret(context.Context, string, string, string) error {
	return nil
}

type fakeTokenService struct{}

func (fakeTokenService) GetAccessToken(context.Context, string) (string, error) { return "", nil }
func (fakeTokenService) RefreshAccessToken(context.Context, string) (string, error) {
	return "", nil
}

func TestRegister_WechatAppManagementRoutesNotRegisteredWithoutAdminMiddlewares(t *testing.T) {
	engine := newIDPRouter(t, nil, &fakeWechatAppService{})

	health := httptest.NewRecorder()
	healthReq := httptest.NewRequest(http.MethodGet, "/api/v2/idp/health", nil)
	engine.ServeHTTP(health, healthReq)
	require.Equal(t, http.StatusOK, health.Code)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/idp/wechat-apps", nil)
	engine.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestRegister_WechatAppListRequiresAdminMiddleware(t *testing.T) {
	engine := newIDPRouter(t, []gin.HandlerFunc{requireAdminHeader()}, &fakeWechatAppService{
		listResult: []*appsvc.WechatAppResult{
			{
				ID:     "1001",
				AppID:  "wx-admin",
				Name:   "Admin App",
				Type:   domain.MP,
				Status: domain.StatusEnabled,
			},
		},
	})

	unauthorized := httptest.NewRecorder()
	unauthorizedReq := httptest.NewRequest(http.MethodGet, "/api/v2/idp/wechat-apps", nil)
	engine.ServeHTTP(unauthorized, unauthorizedReq)
	require.Equal(t, http.StatusUnauthorized, unauthorized.Code)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/idp/wechat-apps?type=MP&status=Enabled", nil)
	req.Header.Set("X-Admin", "1")
	engine.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeAPIResponse[idpresponse.WechatAppListResponse](t, recorder)
	require.Equal(t, 1, body.Total)
	require.Len(t, body.Items, 1)
	require.Equal(t, "wx-admin", body.Items[0].AppID)
}

func TestRegister_WechatAppUpdateEnableDisableRoutes(t *testing.T) {
	service := &fakeWechatAppService{
		updateResult: &appsvc.WechatAppResult{
			ID:     "2001",
			AppID:  "wx-update",
			Name:   "Updated",
			Type:   domain.MP,
			Status: domain.StatusEnabled,
		},
		enableResult: &appsvc.WechatAppResult{
			ID:     "2002",
			AppID:  "wx-enable",
			Name:   "Enabled",
			Type:   domain.MiniProgram,
			Status: domain.StatusEnabled,
		},
		disableResult: &appsvc.WechatAppResult{
			ID:     "2003",
			AppID:  "wx-disable",
			Name:   "Disabled",
			Type:   domain.MP,
			Status: domain.StatusDisabled,
		},
	}
	engine := newIDPRouter(t, []gin.HandlerFunc{requireAdminHeader()}, service)

	updateRecorder := httptest.NewRecorder()
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v2/idp/wechat-apps/wx-update", bytes.NewBufferString(`{"name":"Updated","type":"MP"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("X-Admin", "1")
	engine.ServeHTTP(updateRecorder, updateReq)

	require.Equal(t, http.StatusOK, updateRecorder.Code)
	require.Equal(t, "wx-update", service.lastUpdateAppID)
	require.NotNil(t, service.lastUpdateDTO.Name)
	require.Equal(t, "Updated", *service.lastUpdateDTO.Name)
	require.NotNil(t, service.lastUpdateDTO.Type)
	require.Equal(t, domain.MP, *service.lastUpdateDTO.Type)

	enableRecorder := httptest.NewRecorder()
	enableReq := httptest.NewRequest(http.MethodPost, "/api/v2/idp/wechat-apps/wx-enable/enable", nil)
	enableReq.Header.Set("X-Admin", "1")
	engine.ServeHTTP(enableRecorder, enableReq)
	require.Equal(t, http.StatusOK, enableRecorder.Code)
	require.Equal(t, "wx-enable", service.lastEnableAppID)

	disableRecorder := httptest.NewRecorder()
	disableReq := httptest.NewRequest(http.MethodPost, "/api/v2/idp/wechat-apps/wx-disable/disable", nil)
	disableReq.Header.Set("X-Admin", "1")
	engine.ServeHTTP(disableRecorder, disableReq)
	require.Equal(t, http.StatusOK, disableRecorder.Code)
	require.Equal(t, "wx-disable", service.lastDisableAppID)
}

func TestRegister_WechatAppUpdateRejectsInvalidType(t *testing.T) {
	engine := newIDPRouter(t, []gin.HandlerFunc{requireAdminHeader()}, &fakeWechatAppService{})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v2/idp/wechat-apps/wx-invalid", bytes.NewBufferString(`{"type":"OfficialAccount"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin", "1")
	engine.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var body core.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, code.ErrWechatAppTypeInvalid, body.Code)
}

func newIDPRouter(t *testing.T, middlewares []gin.HandlerFunc, appService appsvc.WechatAppApplicationService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	Register(engine, Dependencies{
		WechatAppHandler: handler.NewWechatAppHandler(appService, fakeCredentialService{}, fakeTokenService{}),
		AdminMiddlewares: middlewares,
	})
	return engine
}

func requireAdminHeader() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("X-Admin") != "1" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "unauthorized",
			})
			return
		}
		c.Next()
	}
}

func decodeAPIResponse[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, 0, envelope.Code)
	require.Equal(t, "success", envelope.Message)

	var payload T
	require.NoError(t, json.Unmarshal(envelope.Data, &payload))
	return payload
}

func TestRegister_WechatAppListPropagatesAppServiceFilter(t *testing.T) {
	service := &fakeWechatAppService{
		listResult: []*appsvc.WechatAppResult{},
	}
	engine := newIDPRouter(t, []gin.HandlerFunc{requireAdminHeader()}, service)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/idp/wechat-apps?type=MP&status=Disabled", nil)
	req.Header.Set("X-Admin", "1")
	engine.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotNil(t, service.lastListFilter.Type)
	require.NotNil(t, service.lastListFilter.Status)
	require.Equal(t, domain.MP, *service.lastListFilter.Type)
	require.Equal(t, domain.StatusDisabled, *service.lastListFilter.Status)
}

func TestRegister_WechatAppListHandlesServiceErrors(t *testing.T) {
	service := &fakeWechatAppServiceWithError{
		err: perrors.WithCode(code.ErrWechatAppStatusInvalid, "boom"),
	}
	engine := newIDPRouterWithError(t, []gin.HandlerFunc{requireAdminHeader()}, service)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/idp/wechat-apps?status=Enabled", nil)
	req.Header.Set("X-Admin", "1")
	engine.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestRegister_WechatAppMutationRoutesPropagateServiceErrors(t *testing.T) {
	engine := newIDPRouterWithError(t, []gin.HandlerFunc{requireAdminHeader()}, &fakeWechatAppServiceWithError{
		err: perrors.WithCode(code.ErrWechatAppNotFound, "missing"),
	})

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "update",
			method: http.MethodPatch,
			path:   "/api/v2/idp/wechat-apps/wx-missing",
			body:   `{"name":"Missing"}`,
		},
		{
			name:   "enable",
			method: http.MethodPost,
			path:   "/api/v2/idp/wechat-apps/wx-missing/enable",
		},
		{
			name:   "disable",
			method: http.MethodPost,
			path:   "/api/v2/idp/wechat-apps/wx-missing/disable",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Admin", "1")

			engine.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusNotFound, recorder.Code)
			var body core.Response
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			require.Equal(t, code.ErrWechatAppNotFound, body.Code)
		})
	}
}

type fakeWechatAppServiceWithError struct {
	err error
}

func (f *fakeWechatAppServiceWithError) CreateApp(context.Context, appsvc.CreateWechatAppDTO) (*appsvc.WechatAppResult, error) {
	return nil, f.err
}

func (f *fakeWechatAppServiceWithError) GetApp(context.Context, string) (*appsvc.WechatAppResult, error) {
	return nil, f.err
}

func (f *fakeWechatAppServiceWithError) ListApps(context.Context, appsvc.ListWechatAppsFilter) ([]*appsvc.WechatAppResult, error) {
	return nil, f.err
}

func (f *fakeWechatAppServiceWithError) UpdateApp(context.Context, string, appsvc.UpdateWechatAppDTO) (*appsvc.WechatAppResult, error) {
	return nil, f.err
}

func (f *fakeWechatAppServiceWithError) EnableApp(context.Context, string) (*appsvc.WechatAppResult, error) {
	return nil, f.err
}

func (f *fakeWechatAppServiceWithError) DisableApp(context.Context, string) (*appsvc.WechatAppResult, error) {
	return nil, f.err
}

func newIDPRouterWithError(t *testing.T, middlewares []gin.HandlerFunc, appService appsvc.WechatAppApplicationService) *gin.Engine {
	return newIDPRouter(t, middlewares, appService)
}
