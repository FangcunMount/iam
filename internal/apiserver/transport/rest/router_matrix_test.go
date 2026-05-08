package rest

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
	authhandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authn/handler"
	authzhandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authz/handler"
	uchandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/identity/handler"
	idphandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/idp/handler"
)

func TestRouterRouteMatrixIncludesKeyPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	NewRouter(routeMatrixDeps()).RegisterRoutes(engine)
	routes := engine.Routes()

	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/health"},
		{http.MethodGet, "/.well-known/jwks.json"},
		{http.MethodPost, "/api/v2/authn/login"},
		{http.MethodPost, "/api/v2/authn/login/prep/phone-otp"},
		{http.MethodPost, "/api/v2/authn/refresh_token"},
		{http.MethodPost, "/api/v2/authn/signups/wechat-miniprogram"},
		{http.MethodPost, "/api/v2/internal/authn/mock-consumers/ensure"},
		{http.MethodGet, "/api/v2/authz/health"},
		{http.MethodPost, "/api/v2/authz/check"},
		{http.MethodGet, "/api/v2/authz/roles"},
		{http.MethodGet, "/api/v2/identity/me"},
		{http.MethodGet, "/api/v2/identity/profiles/:id"},
		{http.MethodGet, "/api/v2/identity/profile-links"},
		{http.MethodGet, "/api/v2/idp/health"},
		{http.MethodGet, "/api/v2/idp/wechat-apps"},
		{http.MethodGet, "/api/v2/suggest/profile"},
		{http.MethodGet, "/debug/cache-governance/catalog"},
		{http.MethodPost, "/api/v2/admin/sessions/:sessionId/revoke"},
	} {
		assertRoutePresent(t, routes, route.method, route.path)
	}
	assertRouteAbsent(t, routes, http.MethodPost, "/api/v2/authn/accounts/wechat/register")
	assertRouteAbsent(t, routes, http.MethodPost, "/api/v2/identity/profiles")
	assertRouteAbsent(t, routes, http.MethodPost, "/api/v2/identity/profile-links")
	assertRouteAbsent(t, routes, http.MethodPost, "/api/v2/identity/profile-links/:id/revoke")
}

func TestRouterOpenAPIContractCoversRegisteredPublicRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	NewRouter(routeMatrixDeps()).RegisterRoutes(engine)
	spec := loadRESTOpenAPISpecs(t)

	var missing []string
	for _, route := range engine.Routes() {
		if !routeMustBeDocumented(route) {
			continue
		}
		path := normalizeOpenAPIPath(route.Path)
		method := strings.ToLower(route.Method)
		methods := spec.Paths[path]
		if methods == nil || methods[method] == nil {
			missing = append(missing, route.Method+" "+route.Path+" normalized as "+path)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("OpenAPI is missing registered routes:\n%s", strings.Join(missing, "\n"))
	}
}

func routeMatrixDeps() Deps {
	deps := restDepsForTest()
	deps.Authn = AuthnDeps{
		AuthHandler:         authhandler.NewAuthHandler(nil, nil, nil),
		AccountHandler:      authhandler.NewAccountHandler(nil, nil),
		JWKSHandler:         authhandler.NewJWKSHandler(nil, nil),
		SessionAdminHandler: authhandler.NewSessionAdminHandler(sessionServiceStub{}),
		TokenService:        tokenServiceStub{},
	}
	deps.Authz = AuthzDeps{
		RoleHandler:        authzhandler.NewRoleHandler(nil, nil),
		RoleBindingHandler: authzhandler.NewRoleBindingHandler(nil, nil),
		PolicyHandler:      authzhandler.NewPolicyHandler(nil, nil),
		ResourceHandler:    authzhandler.NewResourceHandler(nil, nil),
		CheckHandler:       authzhandler.NewCheckHandler(nil),
		RouteAuthorization: casbinStub{},
	}
	deps.IDP = IDPDeps{
		WechatAppHandler: idphandler.NewWechatAppHandler(nil, nil, nil),
	}
	deps.User = UserDeps{
		UserHandler:        uchandler.NewUserHandler(nil, nil, nil, nil),
		ProfileHandler:     uchandler.NewProfileHandler(nil, nil),
		ProfileLinkHandler: uchandler.NewProfileLinkHandler(nil),
	}
	deps.Suggest = SuggestDeps{Service: appsuggest.NewService(appsuggest.Config{})}
	deps.ModuleStatus = ModuleStatus{
		ContainerInitialized: true,
		Authn:                true,
		Authz:                true,
		User:                 true,
		IDP:                  true,
		Suggest:              true,
		AuthEnabled:          true,
	}
	normalizeModuleStatusForTest(&deps.ModuleStatus)
	deps.SeedMockAuth = SeedMockAuthOptions{Enabled: true, SharedSecret: "test-secret"}
	deps.DebugCacheGovernance = DebugCacheGovernanceOptions{AppMode: "development"}
	return deps
}

type openAPISpec struct {
	Paths map[string]map[string]any `yaml:"paths"`
}

func loadRESTOpenAPISpecs(t *testing.T) openAPISpec {
	t.Helper()

	root := repoRoot(t)
	paths := map[string]map[string]any{}
	for _, rel := range []string{
		"api/rest/authn.v2.yaml",
		"api/rest/authz.v2.yaml",
		"api/rest/identity.v2.yaml",
		"api/rest/idp.v2.yaml",
		"api/rest/suggest.v2.yaml",
	} {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		var spec openAPISpec
		if err := yaml.Unmarshal(data, &spec); err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for path, methods := range spec.Paths {
			if paths[path] == nil {
				paths[path] = map[string]any{}
			}
			for method, operation := range methods {
				paths[path][strings.ToLower(method)] = operation
			}
		}
	}
	return openAPISpec{Paths: paths}
}

func routeMustBeDocumented(route gin.RouteInfo) bool {
	if route.Path == "/.well-known/jwks.json" || route.Path == "/api/v2/.well-known/jwks.json" {
		return true
	}
	if !strings.HasPrefix(route.Path, "/api/v2/") {
		return false
	}
	for _, prefix := range []string{
		"/api/v2/public/",
		"/api/v2/internal/",
		"/api/v2/admin/sessions/",
		"/api/v2/admin/accounts/",
		"/api/v2/admin/users/",
	} {
		if strings.HasPrefix(route.Path, prefix) {
			return false
		}
	}
	for _, path := range []string{
		"/api/v2/admin/users",
		"/api/v2/admin/statistics",
		"/api/v2/admin/logs",
		"/api/v2/idp/health",
		"/api/v2/authz/health",
	} {
		if route.Path == path {
			return false
		}
	}
	return true
}

func normalizeOpenAPIPath(path string) string {
	path = strings.TrimPrefix(path, "/api/v2")
	path = strings.ReplaceAll(path, ":accountId", "{accountId}")
	path = strings.ReplaceAll(path, ":sessionId", "{sessionId}")
	path = strings.ReplaceAll(path, ":userId", "{userId}")
	path = strings.ReplaceAll(path, ":app_id", "{app_id}")
	path = strings.ReplaceAll(path, ":kid", "{kid}")
	path = strings.ReplaceAll(path, ":key", "{key}")
	path = strings.ReplaceAll(path, ":id", "{id}")
	if path == "" {
		return "/"
	}
	return path
}

func assertRoutePresent(t *testing.T, routes gin.RoutesInfo, method, path string) {
	t.Helper()
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return
		}
	}
	t.Fatalf("route %s %s not registered", method, path)
}

func assertRouteAbsent(t *testing.T, routes gin.RoutesInfo, method, path string) {
	t.Helper()
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			t.Fatalf("route %s %s should not be registered", method, path)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
}
