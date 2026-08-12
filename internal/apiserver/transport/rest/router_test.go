package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	sessionapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/session"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	cachegovernance "github.com/FangcunMount/iam/v2/internal/apiserver/application/cachegovernance"
	readinessapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/readiness"
	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	authhandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authn/handler"
	authzhandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authz/handler"
	uchandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/identity/handler"

	authnMiddleware "github.com/FangcunMount/iam/v2/internal/pkg/middleware/authn"
	genericapiserver "github.com/FangcunMount/iam/v2/internal/pkg/server"
)

func TestRouterRegistersCacheGovernanceDebugRoutesInDevelopmentByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()

	newRouterForTest(restDepsForTest(), RouterOptions{
		DebugCacheGovernance: DebugCacheGovernanceOptions{Environment: genericapiserver.EnvironmentDevelopment},
	}).RegisterRoutes(engine)

	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/catalog", http.StatusOK, true)
	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/overview", http.StatusOK, true)
	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/families/authn.refresh_token", http.StatusOK, true)
	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/families/unknown.family", http.StatusNotFound, true)
}

func TestRouterDoesNotRegisterCacheGovernanceDebugRoutesInProductionByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()

	newRouterForTest(restDepsForTest(), RouterOptions{
		DebugCacheGovernance: DebugCacheGovernanceOptions{Environment: genericapiserver.EnvironmentProduction},
	}).RegisterRoutes(engine)

	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/catalog", http.StatusNotFound, false)
	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/overview", http.StatusNotFound, false)
	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/families/authn.refresh_token", http.StatusNotFound, false)
}

func TestRouterDoesNotRegisterCacheGovernanceDebugRoutesWhenAdminProtectionUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()

	newRouterForTest(restDepsForTest(), RouterOptions{
		DebugCacheGovernance: DebugCacheGovernanceOptions{
			Environment:  genericapiserver.EnvironmentProduction,
			Enabled:      boolPtr(true),
			RequireAdmin: boolPtr(true),
		},
	}).RegisterRoutes(engine)

	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/catalog", http.StatusNotFound, false)
}

func TestRouterForcesAdminProtectionForCacheGovernanceDebugRoutesInProduction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()

	newRouterForTest(restDepsForTest(), RouterOptions{
		DebugCacheGovernance: DebugCacheGovernanceOptions{
			Environment:  genericapiserver.EnvironmentProduction,
			Enabled:      boolPtr(true),
			RequireAdmin: boolPtr(false),
		},
	}).RegisterRoutes(engine)

	assertDebugRouteStatus(t, engine, http.MethodGet, "/debug/cache-governance/catalog", http.StatusNotFound, false)
}

func TestRouterRegistersSeedMockRouteWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	deps := restDepsForTest()
	deps.Authn = AuthnDeps{
		OnboardingHandler: authhandler.NewOnboardingHandler(nil),
	}
	markModuleAvailableForTest(&deps.ModuleStatus, moduleStateAuthn)

	newRouterForTest(deps, RouterOptions{
		SeedMockAuth: SeedMockAuthOptions{Enabled: true, SharedSecret: "test-secret"},
	}).RegisterRoutes(engine)

	assertRouteRegistered(t, engine, http.MethodPost, "/api/v2/internal/authn/mock-consumers/ensure")
}

func TestSeedMockRouteAuthenticatesBeforeValidatingPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	deps := restDepsForTest()
	deps.Authn = AuthnDeps{
		OnboardingHandler: authhandler.NewOnboardingHandler(nil),
	}
	markModuleAvailableForTest(&deps.ModuleStatus, moduleStateAuthn)

	newRouterForTest(deps, RouterOptions{
		SeedMockAuth: SeedMockAuthOptions{Enabled: true, SharedSecret: "expected-secret"},
	}).RegisterRoutes(engine)

	tests := []struct {
		name       string
		secret     string
		wantStatus int
	}{
		{name: "missing secret", wantStatus: http.StatusUnauthorized},
		{name: "wrong secret", secret: "wrong-secret", wantStatus: http.StatusUnauthorized},
		{name: "correct secret invalid payload", secret: "expected-secret", wantStatus: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v2/internal/authn/mock-consumers/ensure",
				strings.NewReader("{}"),
			)
			req.Header.Set("Content-Type", "application/json")
			if tt.secret != "" {
				req.Header.Set(authnMiddleware.SeedMockSecretHeader, tt.secret)
			}

			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
		})
	}
}

func TestRouterSkipsSeedMockRouteWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	deps := restDepsForTest()
	deps.Authn = AuthnDeps{
		OnboardingHandler: authhandler.NewOnboardingHandler(nil),
	}
	markModuleAvailableForTest(&deps.ModuleStatus, moduleStateAuthn)

	newRouterForTest(deps, RouterOptions{
		SeedMockAuth: SeedMockAuthOptions{Enabled: false, SharedSecret: "test-secret"},
	}).RegisterRoutes(engine)

	assertRouteNotRegistered(t, engine, http.MethodPost, "/api/v2/internal/authn/mock-consumers/ensure")
}

func TestRouterRegistersAuthnV2LoginRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	deps := restDepsForTest()
	deps.Authn = AuthnDeps{
		AuthHandler: authhandler.NewAuthHandler(nil, nil, nil),
	}
	markModuleAvailableForTest(&deps.ModuleStatus, moduleStateAuthn)

	newRouterForTest(deps, RouterOptions{}).RegisterRoutes(engine)

	assertRouteRegistered(t, engine, http.MethodPost, "/api/v2/authn/login")
	assertRouteRegistered(t, engine, http.MethodPost, "/api/v2/authn/login")
}

func TestRouterRegistersModuleRoutesFromModuleStateWithoutLegacyBooleans(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	deps := restDepsForTest()
	deps.Authn = AuthnDeps{
		AuthHandler: authhandler.NewAuthHandler(nil, nil, nil),
	}
	markModuleAvailableForTest(&deps.ModuleStatus, moduleStateAuthn)

	NewRouter(deps).RegisterRoutes(engine)

	assertRouteRegistered(t, engine, http.MethodPost, "/api/v2/authn/login")
}

func TestRouterRegistersAuthnSignupRouteAndRetiresOldWechatRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	deps := restDepsForTest()
	deps.Authn = AuthnDeps{
		OnboardingHandler: authhandler.NewOnboardingHandler(nil),
	}
	markModuleAvailableForTest(&deps.ModuleStatus, moduleStateAuthn)

	newRouterForTest(deps, RouterOptions{}).RegisterRoutes(engine)

	assertRouteRegistered(t, engine, http.MethodPost, "/api/v2/authn/signups/wechat-miniprogram")
}

func TestRouterRegistersBaseRoutesBeforeModuleRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	deps := restDepsForTest()
	deps.Authn = AuthnDeps{
		JWKSHandler: authhandler.NewJWKSHandler(nil, nil, nil),
	}
	markModuleAvailableForTest(&deps.ModuleStatus, moduleStateAuthn)

	newRouterForTest(deps, RouterOptions{}).RegisterRoutes(engine)

	assertRouteRegistered(t, engine, http.MethodGet, "/.well-known/jwks.json")
	assertRouteBefore(t, engine, http.MethodGet, "/health", http.MethodGet, "/.well-known/jwks.json")
}

func TestHealthCheckReturnsSanitizedCompatibilitySummary(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	deps := restDepsForTest()
	deps.ModuleStatus.Modules = map[string]ModuleState{
		moduleStateAuthn: {Bootstrapped: true, Available: true},
		moduleStateAuthz: {Bootstrapped: true, Available: false, DegradedReason: "SECRET-SENTINEL"},
	}

	newRouterForTest(deps, RouterOptions{}).RegisterRoutes(engine)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("GET /health should return valid json: %v", err)
	}
	if strings.Contains(recorder.Body.String(), "SECRET-SENTINEL") {
		t.Fatalf("health response leaked degraded reason: %s", recorder.Body.String())
	}
}

func TestReadinessCheckReturns503WhenDraining(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := readinessapp.New(readinessapp.Config{
		ComponentTimeout: 10 * time.Millisecond,
		TotalTimeout:     50 * time.Millisecond,
	}, []readinessapp.Component{{
		Name: "mysql", Required: true, Check: func(context.Context) error { return nil },
	}})
	checker.MarkDraining()
	deps := restDepsForTest()
	deps.Readiness = checker
	engine := gin.New()
	newRouterForTest(deps, RouterOptions{}).RegisterRoutes(engine)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz status = %d, want 503; body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "error") {
		t.Fatalf("GET /readyz leaked error detail: %s", recorder.Body.String())
	}
}

func TestRouterSkipsSeedMockRouteWithoutSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	deps := restDepsForTest()
	deps.Authn = AuthnDeps{
		OnboardingHandler: authhandler.NewOnboardingHandler(nil),
	}
	markModuleAvailableForTest(&deps.ModuleStatus, moduleStateAuthn)

	newRouterForTest(deps, RouterOptions{
		SeedMockAuth: SeedMockAuthOptions{Enabled: true, SharedSecret: ""},
	}).RegisterRoutes(engine)

	assertRouteNotRegistered(t, engine, http.MethodPost, "/api/v2/internal/authn/mock-consumers/ensure")
}

func TestRegisterAdminRoutesRegistersSessionControlRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	deps := restDepsForTest()
	deps.Authn.SessionAdminHandler = authhandler.NewSessionAdminHandler(sessionServiceStub{})
	router := newRouterForTest(deps, RouterOptions{})

	router.registerAdminRoutes(engine, authnMiddleware.NewJWTAuthMiddleware(nil, casbinStub{}))

	assertRouteRegistered(t, engine, http.MethodPost, "/api/v2/admin/sessions/:sessionId/revoke")
	assertRouteRegistered(t, engine, http.MethodPost, "/api/v2/admin/login-identities/:loginIdentityId/sessions/revoke")
	assertRouteRegistered(t, engine, http.MethodPost, "/api/v2/admin/users/:userId/sessions/revoke")
	for _, path := range []string{
		"/api/v2/admin/users",
		"/api/v2/admin/statistics",
		"/api/v2/admin/logs",
	} {
		assertRouteNotRegistered(t, engine, http.MethodGet, path)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want 404", path, rec.Code)
		}
	}
}

func TestRegisterAdminRoutesFailsClosedWithoutAdminProtection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	deps := restDepsForTest()
	deps.Authn.SessionAdminHandler = authhandler.NewSessionAdminHandler(sessionServiceStub{})
	router := newRouterForTest(deps, RouterOptions{})

	router.registerAdminRoutes(engine, nil)

	assertRouteNotRegistered(t, engine, http.MethodPost, "/api/v2/admin/sessions/:sessionId/revoke")
	assertRouteNotRegistered(t, engine, http.MethodPost, "/api/v2/admin/login-identities/:loginIdentityId/sessions/revoke")
	assertRouteNotRegistered(t, engine, http.MethodPost, "/api/v2/admin/users/:userId/sessions/revoke")
}

func TestRouterRegistersIdentityRefsRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	deps := restDepsForTest()
	deps.Authn.TokenService = tokenServiceStub{}
	deps.User = UserDeps{
		UserHandler:        uchandler.NewUserHandler(nil, nil, nil, nil),
		ProfileHandler:     uchandler.NewProfileHandler(nil),
		ProfileLinkHandler: uchandler.NewProfileLinkHandler(nil),
	}
	markModuleAvailableForTest(&deps.ModuleStatus, moduleStateAuthn)
	markModuleAvailableForTest(&deps.ModuleStatus, moduleStateIdentity)

	newRouterForTest(deps, RouterOptions{}).RegisterRoutes(engine)

	assertRouteRegistered(t, engine, http.MethodGet, "/api/v2/identity/profile-links")
	assertRouteRegistered(t, engine, http.MethodGet, "/api/v2/identity/profiles/:id")
	assertRouteNotRegistered(t, engine, http.MethodGet, "/api/v2/identity/profiles/search")
	assertRouteNotRegistered(t, engine, http.MethodPost, "/api/v2/identity/profile-links")
	assertRouteNotRegistered(t, engine, http.MethodPost, "/api/v2/identity/profile-links/:id/revoke")
	assertRouteNotRegistered(t, engine, http.MethodPost, "/api/v2/identity/profiles")
}

func TestRouterSkipsProtectedRoutesWithoutJWTMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	deps := restDepsForTest()
	deps.User = UserDeps{
		UserHandler:        uchandler.NewUserHandler(nil, nil, nil, nil),
		ProfileHandler:     uchandler.NewProfileHandler(nil),
		ProfileLinkHandler: uchandler.NewProfileLinkHandler(nil),
	}
	deps.Authz = AuthzDeps{
		RoleHandler:        authzhandler.NewRoleHandler(nil, nil),
		RoleBindingHandler: authzhandler.NewRoleBindingHandler(nil, nil),
		PolicyHandler:      authzhandler.NewPolicyHandler(nil, nil),
		ResourceHandler:    authzhandler.NewResourceHandler(nil, nil),
		CheckHandler:       authzhandler.NewCheckHandler(nil),
	}
	deps.Suggest.Service = appsuggest.NewService(appsuggest.Config{})
	markModuleAvailableForTest(&deps.ModuleStatus, moduleStateIdentity)
	markModuleAvailableForTest(&deps.ModuleStatus, moduleStateAuthz)
	markModuleAvailableForTest(&deps.ModuleStatus, moduleStateSuggest)

	newRouterForTest(deps, RouterOptions{}).RegisterRoutes(engine)

	assertRouteNotRegistered(t, engine, http.MethodGet, "/api/v2/identity/profile-links")
	assertRouteNotRegistered(t, engine, http.MethodPost, "/api/v2/identity/profiles")
	assertRouteNotRegistered(t, engine, http.MethodGet, "/api/v2/suggest/profile")
	assertRouteNotRegistered(t, engine, http.MethodGet, "/api/v2/authz/roles")
	assertRouteNotRegistered(t, engine, http.MethodGet, "/api/v2/authz/health")
}

func newRouterForTest(deps Deps, options RouterOptions) *Router {
	deps.RouterOptions = options
	if deps.CacheGovernance == nil {
		deps.CacheGovernance = cachegovernance.NewReadService(nil)
	}
	normalizeModuleStatusForTest(&deps.ModuleStatus)
	return NewRouter(deps)
}

func restDepsForTest() Deps {
	return Deps{
		CacheGovernance: cachegovernance.NewReadService(nil),
		ModuleStatus:    moduleStatusForTest(),
	}
}

func moduleStatusForTest() ModuleStatus {
	return ModuleStatus{
		Container: ModuleState{Bootstrapped: true, Available: true},
		Modules:   map[string]ModuleState{},
	}
}

func normalizeModuleStatusForTest(status *ModuleStatus) {
	if status == nil {
		return
	}
	if status.Modules == nil {
		status.Modules = map[string]ModuleState{}
	}
}

func markModuleAvailableForTest(status *ModuleStatus, name string) {
	status.Modules[name] = ModuleState{Bootstrapped: true, Available: true}
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

func assertRouteBefore(t *testing.T, engine *gin.Engine, beforeMethod, beforePath, afterMethod, afterPath string) {
	t.Helper()
	beforeIndex := -1
	afterIndex := -1
	for i, route := range engine.Routes() {
		if route.Method == beforeMethod && route.Path == beforePath {
			beforeIndex = i
		}
		if route.Method == afterMethod && route.Path == afterPath {
			afterIndex = i
		}
	}
	if beforeIndex < 0 || afterIndex < 0 {
		t.Fatalf("cannot compare route order, %s %s index=%d, %s %s index=%d",
			beforeMethod, beforePath, beforeIndex, afterMethod, afterPath, afterIndex)
	}
	if beforeIndex >= afterIndex {
		t.Fatalf("route %s %s index=%d, want before %s %s index=%d",
			beforeMethod, beforePath, beforeIndex, afterMethod, afterPath, afterIndex)
	}
}

type sessionServiceStub struct{}

func (sessionServiceStub) RevokeSession(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (sessionServiceStub) RevokeAllSessionsByLoginIdentity(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (sessionServiceStub) RevokeAllSessionsByUser(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

var _ sessionapp.Revoker = sessionServiceStub{}

type casbinStub struct{}

func (casbinStub) AuthorizeRoute(_ context.Context, _, _, _, _ string) (bool, error) {
	return true, nil
}

func (casbinStub) DirectRoleKeys(_ context.Context, _, _ string) ([]string, error) {
	return []string{"role:admin"}, nil
}

type tokenServiceStub struct{}

func (tokenServiceStub) IssueToken(context.Context, *authentication.Principal) (*tokenapp.TokenPair, error) {
	return nil, nil
}

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
