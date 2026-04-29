package apiserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	sessionapp "github.com/FangcunMount/iam/internal/apiserver/application/authn/session"
	tokenapp "github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	cachegovernance "github.com/FangcunMount/iam/internal/apiserver/application/cachegovernance"
	appsuggest "github.com/FangcunMount/iam/internal/apiserver/application/suggest"
	"github.com/FangcunMount/iam/internal/apiserver/container"
	"github.com/FangcunMount/iam/internal/apiserver/container/assembler"
	authhandler "github.com/FangcunMount/iam/internal/apiserver/interface/authn/restful/handler"
	authzhandler "github.com/FangcunMount/iam/internal/apiserver/interface/authz/restful/handler"
	uchandler "github.com/FangcunMount/iam/internal/apiserver/interface/uc/restful/handler"
	resttransport "github.com/FangcunMount/iam/internal/apiserver/transport/rest"
	authnMiddleware "github.com/FangcunMount/iam/internal/pkg/middleware/authn"
)

func TestRouterRegistersCacheGovernanceDebugRoutesInDevelopmentByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	c := &container.Container{
		CacheGovernanceService: cachegovernance.NewReadService(nil),
	}

	newRouterForTest(c, resttransport.RouterOptions{
		DebugCacheGovernance: resttransport.DebugCacheGovernanceOptions{AppMode: "development"},
	}).RegisterRoutes(engine)

	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/catalog", http.StatusOK, true)
	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/overview", http.StatusOK, true)
	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/families/authn.refresh_token", http.StatusOK, true)
	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/families/unknown.family", http.StatusNotFound, true)
}

func TestRouterDoesNotRegisterCacheGovernanceDebugRoutesInProductionByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	c := &container.Container{
		CacheGovernanceService: cachegovernance.NewReadService(nil),
	}

	newRouterForTest(c, resttransport.RouterOptions{
		DebugCacheGovernance: resttransport.DebugCacheGovernanceOptions{AppMode: "production"},
	}).RegisterRoutes(engine)

	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/catalog", http.StatusNotFound, false)
	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/overview", http.StatusNotFound, false)
	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/families/authn.refresh_token", http.StatusNotFound, false)
}

func TestRouterDoesNotRegisterCacheGovernanceDebugRoutesWhenAdminProtectionUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	c := &container.Container{
		CacheGovernanceService: cachegovernance.NewReadService(nil),
	}

	newRouterForTest(c, resttransport.RouterOptions{
		DebugCacheGovernance: resttransport.DebugCacheGovernanceOptions{
			AppMode:      "production",
			Enabled:      boolPtr(true),
			RequireAdmin: boolPtr(true),
		},
	}).RegisterRoutes(engine)

	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/catalog", http.StatusNotFound, false)
}

func TestRouterForcesAdminProtectionForCacheGovernanceDebugRoutesInProduction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	c := &container.Container{
		CacheGovernanceService: cachegovernance.NewReadService(nil),
	}

	newRouterForTest(c, resttransport.RouterOptions{
		DebugCacheGovernance: resttransport.DebugCacheGovernanceOptions{
			AppMode:      "production",
			Enabled:      boolPtr(true),
			RequireAdmin: boolPtr(false),
		},
	}).RegisterRoutes(engine)

	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/catalog", http.StatusNotFound, false)
}

func TestRouterRegistersSeedMockRouteWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	c := &container.Container{
		AuthnModule: &assembler.AuthnModule{
			AccountHandler: authhandler.NewAccountHandler(nil, nil),
		},
	}

	newRouterForTest(c, resttransport.RouterOptions{
		SeedMockAuth: resttransport.SeedMockAuthOptions{Enabled: true, SharedSecret: "test-secret"},
	}).RegisterRoutes(engine)

	assertRouteRegistered(t, engine, http.MethodPost, "/api/v1/internal/authn/mock-consumers/ensure")
}

func TestRouterRegistersAuthnV2LoginRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	c := &container.Container{
		AuthnModule: &assembler.AuthnModule{
			AuthHandler: authhandler.NewAuthHandler(nil, nil, nil),
		},
	}

	newRouterForTest(c, resttransport.RouterOptions{}).RegisterRoutes(engine)

	assertRouteRegistered(t, engine, http.MethodPost, "/api/v1/authn/login")
	assertRouteRegistered(t, engine, http.MethodPost, "/api/v2/authn/login")
}

func TestRouterSkipsSeedMockRouteWithoutSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	c := &container.Container{
		AuthnModule: &assembler.AuthnModule{
			AccountHandler: authhandler.NewAccountHandler(nil, nil),
		},
	}

	newRouterForTest(c, resttransport.RouterOptions{
		SeedMockAuth: resttransport.SeedMockAuthOptions{Enabled: true, SharedSecret: ""},
	}).RegisterRoutes(engine)

	assertRouteNotRegistered(t, engine, http.MethodPost, "/api/v1/internal/authn/mock-consumers/ensure")
}

func TestRegisterAdminRoutesRegistersSessionControlRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	router := newRouterForTest(&container.Container{
		AuthnModule: &assembler.AuthnModule{
			SessionAdminHandler: authhandler.NewSessionAdminHandler(sessionServiceStub{}),
		},
	}, resttransport.RouterOptions{})

	router.registerAdminRoutes(engine, authnMiddleware.NewJWTAuthMiddleware(nil, casbinStub{}))

	assertRouteRegistered(t, engine, http.MethodPost, "/api/v1/admin/sessions/:sessionId/revoke")
	assertRouteRegistered(t, engine, http.MethodPost, "/api/v1/admin/accounts/:accountId/sessions/revoke")
	assertRouteRegistered(t, engine, http.MethodPost, "/api/v1/admin/users/:userId/sessions/revoke")
}

func TestRegisterAdminRoutesFailsClosedWithoutAdminProtection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	router := newRouterForTest(&container.Container{
		AuthnModule: &assembler.AuthnModule{
			SessionAdminHandler: authhandler.NewSessionAdminHandler(sessionServiceStub{}),
		},
	}, resttransport.RouterOptions{})

	router.registerAdminRoutes(engine, nil)

	assertRouteNotRegistered(t, engine, http.MethodPost, "/api/v1/admin/sessions/:sessionId/revoke")
	assertRouteNotRegistered(t, engine, http.MethodPost, "/api/v1/admin/accounts/:accountId/sessions/revoke")
	assertRouteNotRegistered(t, engine, http.MethodPost, "/api/v1/admin/users/:userId/sessions/revoke")
}

func TestRouterRegistersIdentityGuardiansRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	c := &container.Container{
		AuthnModule: &assembler.AuthnModule{
			TokenService: tokenServiceStub{},
		},
		UserModule: &assembler.UserModule{
			UserHandler:         uchandler.NewUserHandler(nil, nil, nil, nil),
			ChildHandler:        uchandler.NewChildHandler(nil, nil, nil),
			GuardianshipHandler: uchandler.NewGuardianshipHandler(nil),
		},
	}

	newRouterForTest(c, resttransport.RouterOptions{}).RegisterRoutes(engine)

	assertRouteRegistered(t, engine, http.MethodGet, "/api/v1/identity/guardians")
	assertRouteRegistered(t, engine, http.MethodPost, "/api/v1/identity/guardians/grant")
	assertRouteRegistered(t, engine, http.MethodPost, "/api/v1/identity/children/register")
}

func TestRouterSkipsProtectedRoutesWithoutJWTMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	c := &container.Container{
		UserModule: &assembler.UserModule{
			UserHandler:         uchandler.NewUserHandler(nil, nil, nil, nil),
			ChildHandler:        uchandler.NewChildHandler(nil, nil, nil),
			GuardianshipHandler: uchandler.NewGuardianshipHandler(nil),
		},
		AuthzModule: &assembler.AuthzModule{
			RoleHandler:       authzhandler.NewRoleHandler(nil, nil),
			AssignmentHandler: authzhandler.NewAssignmentHandler(nil, nil),
			PolicyHandler:     authzhandler.NewPolicyHandler(nil, nil),
			ResourceHandler:   authzhandler.NewResourceHandler(nil, nil),
			CheckHandler:      authzhandler.NewCheckHandler(nil),
		},
		SuggestModule: &assembler.SuggestModule{
			Service: appsuggest.NewService(appsuggest.Config{}),
		},
	}

	newRouterForTest(c, resttransport.RouterOptions{}).RegisterRoutes(engine)

	assertRouteNotRegistered(t, engine, http.MethodGet, "/api/v1/identity/guardians")
	assertRouteNotRegistered(t, engine, http.MethodPost, "/api/v1/identity/children/register")
	assertRouteNotRegistered(t, engine, http.MethodGet, "/api/v1/suggest/child")
	assertRouteNotRegistered(t, engine, http.MethodGet, "/api/v1/authz/roles")
	assertRouteNotRegistered(t, engine, http.MethodGet, "/api/v1/authz/health")
}

func newRouterForTest(c *container.Container, options resttransport.RouterOptions) *Router {
	return NewRouter(c.BuildRESTDeps(options))
}

func boolPtr(v bool) *bool {
	return &v
}

func assertDebugRouteStatus(t *testing.T, engine *gin.Engine, method, path string, wantStatus int, wantJSON bool) {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d; body=%s", method, path, recorder.Code, wantStatus, recorder.Body.String())
	}

	if wantJSON && !json.Valid(recorder.Body.Bytes()) {
		t.Fatalf("%s %s should return valid json, got %q", method, path, recorder.Body.String())
	}
}

func assertRouteRegistered(t *testing.T, engine *gin.Engine, method, path string) {
	t.Helper()
	for _, route := range engine.Routes() {
		if route.Method == method && route.Path == path {
			return
		}
	}
	t.Fatalf("route %s %s not registered", method, path)
}

func assertRouteNotRegistered(t *testing.T, engine *gin.Engine, method, path string) {
	t.Helper()
	for _, route := range engine.Routes() {
		if route.Method == method && route.Path == path {
			t.Fatalf("route %s %s should not be registered", method, path)
		}
	}
}

type sessionServiceStub struct{}

func (sessionServiceStub) RevokeSession(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (sessionServiceStub) RevokeAllSessionsByAccount(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (sessionServiceStub) RevokeAllSessionsByUser(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

var _ sessionapp.SessionApplicationService = sessionServiceStub{}

type casbinStub struct{}

func (casbinStub) Enforce(_ context.Context, _, _, _, _ string) (bool, error) {
	return true, nil
}

func (casbinStub) GetRolesForUser(_ context.Context, _, _ string) ([]string, error) {
	return []string{"role:admin"}, nil
}

type tokenServiceStub struct{}

func (tokenServiceStub) IssueServiceToken(context.Context, tokenapp.IssueServiceTokenRequest) (*tokenapp.TokenIssueResult, error) {
	return nil, nil
}

func (tokenServiceStub) RefreshToken(context.Context, string) (*tokenapp.TokenRefreshResult, error) {
	return nil, nil
}

func (tokenServiceStub) RevokeAccessToken(context.Context, string) error {
	return nil
}

func (tokenServiceStub) RevokeRefreshToken(context.Context, string) error {
	return nil
}

func (tokenServiceStub) VerifyToken(context.Context, tokenapp.VerifyTokenRequest) (*tokenapp.TokenVerifyResult, error) {
	return &tokenapp.TokenVerifyResult{Valid: true}, nil
}

var _ tokenapp.TokenApplicationService = tokenServiceStub{}
