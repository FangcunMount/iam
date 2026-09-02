package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/FangcunMount/iam/v3/pkg/eventcatalog"
)

const modulePath = "github.com/FangcunMount/iam/v3/"

var activeLegacyApplicationInfrastructureImports = map[string]string{}

var retiredArchitectureExceptionReasonParts = [][]string{
	{"application", "test", "support"},
	{"legacy", "uow", "factory", "to", "invert", "in", "phase", "3"},
}

func TestTransportLoggersRemainMetadataOnly(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	files := []string{
		"internal/pkg/middleware/api_logger.go",
		"internal/pkg/middleware/grpc_server_logger.go",
	}
	forbidden := []string{
		"Request.Body",
		"io.ReadAll",
		"protojson.",
		"Authorization",
		"Cookie",
		"Query()",
		"RawQuery",
	}
	for _, rel := range files {
		content, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, fragment := range forbidden {
			if strings.Contains(string(content), fragment) {
				t.Fatalf("%s contains forbidden payload/header logging fragment %q", rel, fragment)
			}
		}
	}
}

func TestSecuritySensitiveLogsAndGRPCErrorsStayPublicSafe(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	exactGuards := map[string][]string{
		"internal/apiserver/infra/cache/redis/token-store.go": {
			`log.String("key", key)`,
			`log.String("error", err.Error())`,
		},
		"internal/apiserver/infra/cache/redis/accesstoken_cache.go": {
			`log.String("key", key)`,
			`log.String("error", err.Error())`,
		},
		"internal/apiserver/domain/authn/token/refresher.go": {
			"token_hint",
			"MaskToken",
		},
		"internal/apiserver/application/authn/session/sign_out.go": {
			`"error", err.Error()`,
		},
		"internal/apiserver/domain/authn/authentication/authenticator.go": {
			`"error", err.Error()`,
		},
		"internal/apiserver/transport/grpc/service/identity/profile_link_command.go": {
			`Error: err.Error()`,
		},
		"internal/apiserver/transport/grpc/service/idp/service_impl.go": {
			`status.Error(codes.Internal`,
			`err.Error()`,
		},
	}
	for rel, forbidden := range exactGuards {
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range forbidden {
			if strings.Contains(string(source), token) {
				t.Fatalf("%s contains forbidden security-boundary token %q", rel, token)
			}
		}
	}

	if _, err := os.Stat(filepath.Join(root, "internal", "pkg", "security", "sanitize", "token.go")); err == nil {
		t.Fatal("partial token masking helper must not return; credentials are omitted from logs")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}

	mapper, err := os.ReadFile(filepath.Join(root, "internal", "pkg", "grpc", "error_mapper.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		`codes.Internal, codes.Unknown, codes.DataLoss`,
		`return "internal server error"`,
		`return "service unavailable"`,
	} {
		if !strings.Contains(string(mapper), required) {
			t.Fatalf("gRPC mapper is missing safe public mapping %q", required)
		}
	}

	forbiddenServerErrorPatterns := []*regexp.Regexp{
		regexp.MustCompile(`status\.Errorf\(codes\.(Internal|Unknown|DataLoss|Unavailable|DeadlineExceeded|Canceled)`),
		regexp.MustCompile(`status\.Error\(codes\.(Internal|Unknown|DataLoss|Unavailable|DeadlineExceeded|Canceled),\s*err\.Error\(\)`),
	}
	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "transport", "grpc", "service"), func(path, source string) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		for _, pattern := range forbiddenServerErrorPatterns {
			if match := pattern.FindString(source); match != "" {
				rel := filepath.ToSlash(mustRel(t, root, path))
				t.Fatalf("%s exposes a dynamic server-side gRPC error via %q; use internal/pkg/grpc.ToStatusError", rel, match)
			}
		}
	})
}

func TestProductionLoggingAndMaintenanceImageContract(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	productionConfig, err := os.ReadFile(filepath.Join(root, "configs", "apiserver.prod.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"log-level: 1",
		"format: json",
		"enable-color: false",
		"development: false",
	} {
		if !strings.Contains(string(productionConfig), required) {
			t.Fatalf("production configuration is missing %q", required)
		}
	}

	dockerfile, err := os.ReadFile(filepath.Join(root, "build", "docker", "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"./cmd/iam-maintenance/",
		"/app/iam-maintenance",
		"localhost:9080/healthz",
	} {
		if !strings.Contains(string(dockerfile), required) {
			t.Fatalf("production image contract is missing %q", required)
		}
	}
}

func TestDomainPackagesDoNotAddInfrastructureDependencies(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)

	scanImports(t, filepath.Join(root, "internal", "apiserver", "domain"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, imp := range imports {
			if !isDomainForbiddenImport(imp) {
				continue
			}
			t.Fatalf("%s imports %s; domain must stay independent from infrastructure and database packages", rel, imp)
		}
	})
}

func TestMySQLLoginIdentityDoesNotDependOnApplicationLinking(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	forbidden := modulePath + "internal/apiserver/application/authn/linking"
	scanImportsIncludingTests(t, filepath.Join(root, "internal", "apiserver", "infra", "mysql", "loginidentity"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, imp := range imports {
			if imp == forbidden {
				t.Fatalf("%s imports %s; login identity infrastructure must depend on domain ports only", rel, imp)
			}
		}
	})
}

func TestApplicationPackagesDoNotAddTransportOrInfrastructureDependencies(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	assertAllowlistReasons(t, activeLegacyApplicationInfrastructureImports)
	assertNoRetiredArchitectureExceptionReasons(t, activeLegacyApplicationInfrastructureImports)
	seen := map[string]struct{}{}

	scanImports(t, filepath.Join(root, "internal", "apiserver", "application"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		if isApplicationTestutilPath(rel) {
			return
		}
		for _, imp := range imports {
			if strings.HasPrefix(imp, modulePath+"internal/apiserver/interface/") ||
				strings.HasPrefix(imp, modulePath+"internal/apiserver/transport/") {
				t.Fatalf("%s imports %s; application layer must not depend on transport implementations", rel, imp)
			}
			if !isApplicationForbiddenImport(imp) {
				continue
			}
			key := rel + ":" + imp
			if _, ok := activeLegacyApplicationInfrastructureImports[key]; ok {
				seen[key] = struct{}{}
				continue
			}
			t.Fatalf("%s imports %s; add a port/factory seam or document a temporary allowlist reason", rel, imp)
		}
	})
	assertAllowlistStillUsed(t, activeLegacyApplicationInfrastructureImports, seen)
}

func TestApplicationTestSupportDependenciesStayInTestutil(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	scanImports(t, filepath.Join(root, "internal", "apiserver", "application"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		if !isApplicationTestutilPath(rel) {
			return
		}
		for _, imp := range imports {
			if isApplicationTestutilAllowedImport(imp) {
				continue
			}
			if strings.HasPrefix(imp, modulePath+"internal/apiserver/infra/") ||
				strings.HasPrefix(imp, modulePath+"internal/pkg/database") ||
				strings.HasPrefix(imp, modulePath+"internal/pkg/migration") ||
				imp == "gorm.io/gorm" ||
				strings.HasPrefix(imp, "gorm.io/driver/") {
				t.Fatalf("%s imports %s; application test support may only depend on GORM and infra/mysql test PO helpers", rel, imp)
			}
		}
	})
}

func TestRESTRouterDoesNotImportCompositionOrGlobalConfig(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	scanImports(t, filepath.Join(root, "internal", "apiserver", "transport", "rest"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, imp := range imports {
			if strings.HasPrefix(imp, modulePath+"internal/apiserver/container") || imp == "github.com/spf13/viper" {
				t.Fatalf("%s imports %s; REST router must consume transport deps instead of composition container or global config", rel, imp)
			}
		}
	})
}

func TestRuntimeCompositionDoesNotReadGlobalConfigDirectly(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, relRoot := range []string{
		"internal/apiserver/application",
		"internal/apiserver/container",
	} {
		scanImports(t, filepath.Join(root, relRoot), func(path string, imports []string) {
			rel := filepath.ToSlash(mustRel(t, root, path))
			for _, imp := range imports {
				if imp == "github.com/spf13/viper" {
					t.Fatalf("%s imports %s; runtime config must enter via typed options/deps", rel, imp)
				}
			}
		})
	}
}

func containerModuleRoots() []string {
	return []string{
		"identity",
		"authn",
		"authz",
		"idp",
		"suggest",
	}
}

func scanContainerModuleSources(t *testing.T, visit func(path, source string)) {
	t.Helper()
	root := repoRoot(t)
	for _, mod := range containerModuleRoots() {
		scanGoSources(t, filepath.Join(root, "internal", "apiserver", "container", mod), visit)
	}
}

func scanContainerModuleImports(t *testing.T, visit func(path string, imports []string)) {
	t.Helper()
	root := repoRoot(t)
	for _, mod := range containerModuleRoots() {
		scanImports(t, filepath.Join(root, "internal", "apiserver", "container", mod), visit)
	}
}

func scanContainerModuleGoFiles(t *testing.T, visit func(path string, file *ast.File)) {
	t.Helper()
	root := repoRoot(t)
	for _, mod := range containerModuleRoots() {
		scanGoFiles(t, filepath.Join(root, "internal", "apiserver", "container", mod), visit)
	}
}

func isContainerModulePackage(rel string) bool {
	for _, mod := range containerModuleRoots() {
		if strings.HasPrefix(rel, "internal/apiserver/container/"+mod+"/") {
			return true
		}
	}
	return strings.HasPrefix(rel, "internal/apiserver/container/platform/")
}

func TestAssemblerModulesUseTypedDependencies(t *testing.T) {
	t.Parallel()

	scanContainerModuleSources(t, func(path, source string) {
		root := repoRoot(t)
		rel := filepath.ToSlash(mustRel(t, root, path))
		if strings.Contains(source, "params ...interface{}") {
			t.Fatalf("%s uses variadic interface module initialization; use typed deps instead", rel)
		}
		if strings.Contains(source, "[]interface{}") {
			t.Fatalf("%s uses []interface{} for module dependencies; use typed deps instead", rel)
		}
	})
}

func TestAssemblerComponentHoldersDoNotReturn(t *testing.T) {
	t.Parallel()

	forbiddenTokens := []string{
		"type Module interface",
		"type ModuleInfo struct",
		"type RepoComponent struct",
		"type ServiceComponent struct",
		"type HandlerComponent struct",
		"Repository  interface{}",
		"Service     interface{}",
		"Handler     interface{}",
	}
	scanContainerModuleSources(t, func(path, source string) {
		root := repoRoot(t)
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, token := range forbiddenTokens {
			if strings.Contains(source, token) {
				t.Fatalf("%s contains retired assembler component holder %q", rel, token)
			}
		}
	})
}

func TestAssemblerDoesNotConstructTransportImplementations(t *testing.T) {
	t.Parallel()

	scanContainerModuleImports(t, func(path string, imports []string) {
		root := repoRoot(t)
		rel := filepath.ToSlash(mustRel(t, root, path))
		if strings.HasSuffix(rel, "/rest.go") || strings.HasSuffix(rel, "/grpc.go") {
			return
		}
		for _, imp := range imports {
			if strings.HasPrefix(imp, modulePath+"internal/apiserver/transport/") {
				t.Fatalf("%s imports %s; module assembly must expose application/domain capabilities and leave REST/gRPC construction to module collectors", rel, imp)
			}
		}
	})
}

func TestAssemblerModulesDoNotExposeTransportFields(t *testing.T) {
	t.Parallel()

	forbidden := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s*\w*Handler\s+\*`),
		regexp.MustCompile(`(?m)^\s*GRPCService\s+\*`),
	}
	scanContainerModuleSources(t, func(path, source string) {
		root := repoRoot(t)
		rel := filepath.ToSlash(mustRel(t, root, path))
		if strings.HasSuffix(rel, "/rest.go") || strings.HasSuffix(rel, "/grpc.go") {
			return
		}
		for _, pattern := range forbidden {
			if match := pattern.FindString(source); match != "" {
				t.Fatalf("%s exposes transport field %q; expose application/domain capability methods instead", rel, strings.TrimSpace(match))
			}
		}
	})
}

func TestAuthnModuleDoesNotExposeConcreteApplicationFields(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	source, err := os.ReadFile(filepath.Join(root, "internal", "apiserver", "container", "authn", "module.go"))
	if err != nil {
		t.Fatal(err)
	}
	forbidden := regexp.MustCompile(`(?m)^\s*(SessionService|SessionRevoker|TokenService|KeyManagementApp|KeyPublishApp|KeyRotationApp)\s+`)
	if match := forbidden.FindString(string(source)); match != "" {
		t.Fatalf("AuthnModule exposes concrete application field %q; use ApplicationCapabilities instead", strings.TrimSpace(match))
	}
}

func TestAuthnConsumersDependOnNarrowCapabilities(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	tests := []struct {
		path      string
		required  []string
		forbidden []string
	}{
		{
			path:      "internal/apiserver/application/authn/signin/deps.go",
			required:  []string{"tokenapp.AuthenticationGrantIssuer"},
			forbidden: []string{"TokenApplicationService", "tokenapp.Capabilities", "AdmissionPolicy", "SessionCreator", "TokenSetMinter"},
		},
		{
			path:      "internal/apiserver/application/authn/session/service.go",
			required:  []string{"tokenapp.Refresher", "tokenapp.Revoker"},
			forbidden: []string{"TokenApplicationService", "tokenapp.Capabilities"},
		},
		{
			path:      "internal/pkg/middleware/authn/jwt_middleware.go",
			required:  []string{"token.Verifier"},
			forbidden: []string{"TokenApplicationService", "token.Capabilities"},
		},
	}

	for _, tt := range tests {
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(tt.path)))
		if err != nil {
			t.Fatalf("read %s: %v", tt.path, err)
		}
		text := string(source)
		for _, fragment := range tt.required {
			if !strings.Contains(text, fragment) {
				t.Fatalf("%s must depend on narrow token capability %q", tt.path, fragment)
			}
		}
		for _, fragment := range tt.forbidden {
			if strings.Contains(text, fragment) {
				t.Fatalf("%s depends on broad token abstraction %q", tt.path, fragment)
			}
		}
	}
}

func TestAuthnGrantOwnsAdmissionAndSessionTokenCoordination(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	assertFileContains(t, root, "internal/apiserver/domain/authn/grant/grant.go", "type AuthenticationGrant struct")
	assertFileContains(t, root, "internal/apiserver/domain/authn/grant/issuer.go", "admissiondomain.Require(")
	assertFileContains(t, root, "internal/apiserver/domain/authn/grant/issuer.go", "s.sessionCreator.Create(")
	assertFileContains(t, root, "internal/apiserver/domain/authn/grant/issuer.go", "s.tokenSetMinter.MintTokenSet(")

	tokenSource, err := os.ReadFile(filepath.Join(root, "internal", "apiserver", "domain", "authn", "token", "token.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(tokenSource), "type AuthenticationGrant struct") {
		t.Fatal("domain/authn/token must not own AuthenticationGrant; Session + TokenSet coordination belongs to domain/authn/grant")
	}
}

func TestAuthnAdmissionPolicyDoesNotRegressToSubjectAccessSessionModel(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, relRoot := range []string{
		"internal/apiserver/domain/authn",
		"internal/apiserver/application/authn",
		"internal/apiserver/container/authn",
	} {
		scanGoSources(t, filepath.Join(root, filepath.FromSlash(relRoot)), func(path, source string) {
			for _, retired := range []string{"SubjectAccessEvaluator", "SubjectAccessDecision", "subjectaccess"} {
				if strings.Contains(source, retired) {
					rel := filepath.ToSlash(mustRel(t, root, path))
					t.Fatalf("%s contains retired authentication-admission concept %q", rel, retired)
				}
			}
		})
	}

	assertFileContains(t, root, "internal/apiserver/domain/authn/admission/policy.go", "type Policy interface")
	assertFileContains(t, root, "internal/apiserver/domain/authn/admission/require.go", "func Require(")
	assertFileContains(t, root, "internal/apiserver/application/authn/admission/guard.go", "func MapError(")
	assertFileContains(t, root, "internal/apiserver/domain/authn/grant/issuer.go", "admissiondomain.Require(")
}

func TestRESTRegistrarsDoNotUsePackageGlobalDependencies(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "transport", "rest"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		if strings.Contains(source, "var deps") {
			t.Fatalf("%s declares package-level deps; pass dependencies explicitly to Register", rel)
		}
		if strings.Contains(source, "func Provide(") {
			t.Fatalf("%s exposes Provide; pass dependencies explicitly to Register", rel)
		}
	})
}

func TestRetiredCompatibilityAliasPackagesDoNotReturn(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	retiredPaths := []string{
		"internal/apiserver/infra/cache",
		"internal/apiserver/outboxcore",
		"internal/apiserver/port/outbox",
		"internal/pkg/event",
		"internal/pkg/eventcatalog",
		"internal/pkg/eventcodec",
		"internal/pkg/eventruntime",
	}
	for _, rel := range retiredPaths {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(rel), "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) > 0 {
			t.Fatalf("%s is retired compatibility alias code; use application/cachegovernance or public pkg/event*/pkg/outbox* primitives", rel)
		}
	}

	forbiddenImports := map[string]struct{}{}
	for _, rel := range retiredPaths {
		forbiddenImports[modulePath+filepath.ToSlash(rel)] = struct{}{}
	}
	scanImportsIncludingTests(t, filepath.Join(root, "internal"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		if rel == "internal/pkg/architecture/architecture_test.go" {
			return
		}
		for _, imp := range imports {
			if _, forbidden := forbiddenImports[imp]; forbidden {
				t.Fatalf("%s imports retired compatibility alias %s", rel, imp)
			}
		}
	})
}

func TestApplicationTestsUseTestutilForInfrastructureDependencies(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	scanImportsIncludingTests(t, filepath.Join(root, "internal", "apiserver", "application"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		if !strings.HasSuffix(rel, "_test.go") || isApplicationTestutilPath(rel) {
			return
		}
		for _, imp := range imports {
			if isApplicationForbiddenImport(imp) ||
				imp == "gorm.io/gorm" ||
				strings.HasPrefix(imp, "gorm.io/driver/") {
				t.Fatalf("%s imports %s; application tests must go through application/*/testutil for infra-backed fixtures", rel, imp)
			}
		}
	})
}

func TestIdentityApplicationTransactionScopedCallsUseTxContext(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	forbiddenTokens := []string{
		"managerService.Establish(ctx",
		"managerService.Revoke(ctx",
	}
	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "application", "identity"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		if strings.HasSuffix(rel, "_test.go") {
			return
		}
		for _, token := range forbiddenTokens {
			if strings.Contains(source, token) {
				t.Fatalf("%s uses outer ctx in a transaction-scoped domain call %q; pass txCtx instead", rel, token)
			}
		}
	})
}

func TestRESTRouterTestsUseExplicitDeps(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	rel := filepath.Join("internal", "apiserver", "transport", "rest", "router_test.go")
	path := filepath.Join(root, rel)
	imports := importsForFile(t, path)
	for _, imp := range imports {
		if strings.HasPrefix(imp, modulePath+"internal/apiserver/container") {
			t.Fatalf("%s imports %s; router tests must construct resttransport.Deps directly", filepath.ToSlash(rel), imp)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "BuildRESTDeps(") {
		t.Fatalf("%s calls BuildRESTDeps; router tests must exercise Router's dependency surface directly", filepath.ToSlash(rel))
	}
}

func TestRESTLoginDoesNotOwnAuthMethodDispatch(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	rel := "internal/apiserver/transport/rest/authn/handler/auth_login.go"
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, token := range []string{
		"loginPayloadAdapters",
		"type loginPayloadAdapter",
		"passwordLoginRequest",
		"phoneOTPLoginRequest",
		"wechatLoginRequest",
		"wecomLoginRequest",
	} {
		if strings.Contains(source, token) {
			t.Fatalf("%s contains REST-owned login dispatch %q; public auth methods must come from application/authn/session", rel, token)
		}
	}
	assertFileContains(t, root, rel, "session.BuildExplicitLoginRequest")
}

func TestIDPTokenAppNotFoundUsesStructuredError(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	foundStructuredCode := false
	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "application", "idp", "wechatapp"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		if strings.Contains(source, "code.ErrWechatAppNotFound") {
			foundStructuredCode = true
		}
		for _, token := range []string{
			`fmt.Errorf("wechat app not found`,
			`errors.New("wechat app not found`,
		} {
			if strings.Contains(source, token) {
				t.Fatalf("%s contains unstructured WeChat app not found error %q; use code.ErrWechatAppNotFound", rel, token)
			}
		}
	})
	if !foundStructuredCode {
		t.Fatalf("application/idp/wechatapp does not use code.ErrWechatAppNotFound")
	}
}

func TestGRPCServicesUseSharedCodedErrorMapper(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "transport", "grpc", "service"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, token := range []string{
			"ParseCoder(",
			".HTTPStatus()",
		} {
			if strings.Contains(source, token) {
				t.Fatalf("%s contains private coded-error mapping token %q; use internal/pkg/grpc shared mapper", rel, token)
			}
		}
	})
}

func TestRESTRouteRegistrationUsesModuleStateAvailability(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, rel := range []string{
		"internal/apiserver/transport/rest/router.go",
		"internal/apiserver/transport/rest/module_routes.go",
	} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		for _, token := range []string{
			".ModuleStatus.ContainerInitialized",
			".ModuleStatus.Authn",
			".ModuleStatus.Authz",
			".ModuleStatus.User",
			".ModuleStatus.IDP",
			".ModuleStatus.Suggest",
			".ModuleStatus.AuthEnabled",
		} {
			if strings.Contains(source, token) {
				t.Fatalf("%s reads legacy module status %q during route registration; use ModuleState availability helpers", rel, token)
			}
		}
	}
}

func TestIdentityProfileLinkListDoesNotUseNPlusOneUserLookup(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	rel := "internal/apiserver/transport/grpc/service/identity/profile_link_query.go"
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, token := range []string{
		"GetByID(ctx, g.UserID)",
		"userQuerySvc.GetByID",
	} {
		if strings.Contains(source, token) {
			t.Fatalf("%s uses per-link user lookup %q; use batch user query and preserve edge order", rel, token)
		}
	}
	assertFileContains(t, root, rel, "BatchGetByID")
}

func TestProfileLinkListProfilesDoesNotUseNPlusOneProfileLookup(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	rel := "internal/apiserver/application/identity/profilelink/service_query.go"
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if strings.Contains(source, "tx.Profiles.FindByID(txCtx, g.Profile)") {
		t.Fatalf("%s uses per-link profile lookup in ListProfilesForUser; use batch FindByIDs and preserve link order", rel)
	}
	assertFileContains(t, root, rel, "tx.Profiles.FindByIDs")
}

func TestRootAPIServerPackageOwnsOnlyRunDelegation(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	scanImports(t, filepath.Join(root, "internal", "apiserver"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		if filepath.Dir(rel) != "internal/apiserver" {
			return
		}
		for _, imp := range imports {
			if strings.HasPrefix(imp, modulePath+"internal/apiserver/container") ||
				strings.HasPrefix(imp, modulePath+"internal/apiserver/transport/") ||
				strings.HasPrefix(imp, modulePath+"internal/apiserver/interface/") ||
				imp == "github.com/FangcunMount/component-base/pkg/processruntime" {
				t.Fatalf("%s imports %s; root apiserver package must delegate process ownership to internal/apiserver/process", rel, imp)
			}
		}
	})
}

func TestTransportPackagesDoNotDependOnLegacyInterfacePackages(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, relRoot := range []string{
		"internal/apiserver/transport/rest",
		"internal/apiserver/transport/grpc",
	} {
		scanImports(t, filepath.Join(root, relRoot), func(path string, imports []string) {
			rel := filepath.ToSlash(mustRel(t, root, path))
			for _, imp := range imports {
				if strings.HasPrefix(imp, modulePath+"internal/apiserver/interface/") {
					t.Fatalf("%s imports %s; transport packages must own registration instead of wrapping legacy interface packages", rel, imp)
				}
			}
		})
	}
}

func TestLegacyInterfacePackageIsRetired(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	legacyRoot := filepath.Join(root, "internal", "apiserver", "interface")
	err := filepath.WalkDir(legacyRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel := filepath.ToSlash(mustRel(t, root, path))
		t.Fatalf("%s is a legacy transport implementation; move it under internal/apiserver/transport", rel)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestLocalProcessRunnerDoesNotReturn(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, rel := range []string{
		"internal/apiserver/process/runner.go",
		"internal/apiserver/process/runner_test.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
			t.Fatalf("%s is retired local process runner code; use component-base/pkg/processruntime", rel)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
}

func TestProcessRootDoesNotOwnBootstrapDetails(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	rel := "internal/apiserver/process/root.go"
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, token := range []string{
		"applyGRPCOptions",
		"normalizeNSQConfig",
		"parseIDPEncryptionKey",
		"processruntime.RunGroup",
		"messaging.NewEventBus",
		"base64.",
		"hex.",
		"runOutboxRelay",
	} {
		if strings.Contains(source, token) {
			t.Fatalf("%s contains bootstrap detail %q; keep process root to server lifecycle only", rel, token)
		}
	}
}

func TestContainerCapabilityNavigationStaysInCollectors(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	allowed := map[string]struct{}{
		"internal/apiserver/container/rest_deps.go":       {},
		"internal/apiserver/container/grpc_registry.go":   {},
		"internal/apiserver/container/runtime_deps.go":    {},
		"internal/apiserver/container/module_graph.go":    {},
		"internal/apiserver/container/identity/rest.go":   {},
		"internal/apiserver/container/identity/grpc.go":   {},
		"internal/apiserver/container/authn/rest.go":      {},
		"internal/apiserver/container/authn/grpc.go":      {},
		"internal/apiserver/container/authn/runtime.go":   {},
		"internal/apiserver/container/authz/rest.go":      {},
		"internal/apiserver/container/authz/grpc.go":      {},
		"internal/apiserver/container/authz/runtime.go":   {},
		"internal/apiserver/container/idp/rest.go":        {},
		"internal/apiserver/container/idp/grpc.go":        {},
		"internal/apiserver/container/suggest/rest.go":    {},
		"internal/apiserver/container/suggest/runtime.go": {},
	}
	forbiddenTokens := []string{
		"AuthnModule.AuthHandler",
		"AuthnModule.OnboardingHandler",
		"AuthnModule.JWKSHandler",
		"AuthnModule.SessionAdminHandler",
		"AuthnModule.TokenService",
		"AuthnModule.GRPCService",
		"AuthnModule.RotationScheduler",
		"AuthzModule.RoleHandler",
		"AuthzModule.AssignmentHandler",
		"AuthzModule.PolicyHandler",
		"AuthzModule.ResourceHandler",
		"AuthzModule.CheckHandler",
		"AuthzModule.CasbinAdapter",
		"AuthzModule.GRPCService",
		"IdentityModule.UserHandler",
		"IdentityModule.ProfileHandler",
		"IdentityModule.ProfileLinkHandler",
		"IdentityModule.GRPCService",
		"UserModule.UserHandler",
		"UserModule.ProfileHandler",
		"UserModule.ProfileLinkHandler",
		"UserModule.GRPCService",
		"IDPModule.WechatAppHandler",
		"IDPModule.GRPCService",
		"SuggestModule.Cleanup",
	}

	scanGoSources(t, filepath.Join(root, "internal", "apiserver"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		if _, ok := allowed[rel]; ok {
			return
		}
		if isContainerModulePackage(rel) {
			return
		}
		for _, token := range forbiddenTokens {
			if strings.Contains(source, token) {
				t.Fatalf("%s directly navigates container module capability %q; add a container collector instead", rel, token)
			}
		}
	})
}

func TestRetiredAuthzRuntimeAndV2ContractsDoNotRegress(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, rel := range []string{
		"internal/apiserver/infra/casbin",
		"internal/apiserver/infra/mysql/casbinrule",
		"internal/apiserver/domain/authz/scope",
		"internal/apiserver/application/authz/assignmentauth",
		"internal/apiserver/application/authz/policysync",
		"internal/apiserver/application/authz/rolebinding",
		"internal/apiserver/application/authz/shared",
		"internal/apiserver/domain/authz/rolebinding",
		"internal/apiserver/infra/authz/native",
		"internal/apiserver/infra/mysql/rolebinding",
	} {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(rel), "*"))
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) > 0 {
			t.Fatalf("retired authorization path still contains files: %s", rel)
		}
	}
	for _, rel := range []string{
		"configs/casbin_model.conf",
		"internal/apiserver/infra/authz/runtime/casbin_role_resolver.go",
		"internal/pkg/middleware/authn/metrics.go",
		"internal/pkg/middleware/authn/roles.go",
		"api/rest/authz.v2.yaml",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			if err != nil {
				t.Fatal(err)
			}
			t.Fatalf("retired authorization file still exists: %s", rel)
		}
	}
	if matches, err := filepath.Glob(filepath.Join(root, "api", "grpc", "iam", "authz", "v2", "*")); err != nil {
		t.Fatal(err)
	} else if len(matches) > 0 {
		t.Fatalf("retired AuthZ v2 contract files still exist: %v", matches)
	}
	assertFileContains(t, root, "api/grpc/iam/authz/v3/authz.proto", "OBJECT_CHECK_REQUIRED")
	assertFileContains(t, root, "api/grpc/iam/authz/v3/authz.proto", "oneof value")
	assertFileContains(t, root, "internal/apiserver/infra/authz/runtime/snapshot.go", "BuildSnapshot")
	assertFileContains(t, root, "internal/apiserver/infra/authz/runtime/role_graph.go", "type roleGraph struct")
	assertFileLacks(t, root, "go.mod", "github.com/casbin/")
	assertFileContains(t, root, "internal/apiserver/domain/authz/permissiongrant/grant.go", "Constraint")
	assertFileLacks(t, root, "internal/apiserver/domain/authz/resource/action.go", "type Scope")
	assertFileLacks(t, root, "internal/apiserver/domain/authz/resource/action.go", "ScopeAll")
	assertFileContains(t, root, "web/swagger-ui/swagger-ui-dist/swagger-initializer.js", "/openapi/authz.v3.yaml")
	assertFileLacks(t, root, "web/swagger-ui/swagger-ui-dist/swagger-initializer.js", "authz.v2")
	assertFileContains(t, root, "internal/apiserver/infra/authz/assignmentconstraints/loader.go", "/iam.authz.v3.AuthorizationService/GrantAssignment")
	assertFileContains(t, root, "configs/grpc_acl.yaml", "/iam.authz.v3.AuthorizationService/ReplaceManagedAssignments")
	assertFileLacks(t, root, "internal/apiserver/infra/authz/assignmentconstraints/loader.go", "iam.authz.v2")
	assertFileLacks(t, root, "pkg/sdk/docs/06-authz.md", "iam.authz.v2")
}

func TestAuthnAndAuthzHTTPMiddlewareStaySeparated(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	authnMiddleware := "internal/pkg/middleware/authn/jwt_middleware.go"
	authzMiddleware := "internal/pkg/middleware/authz/middleware.go"
	assertFileContains(t, root, authnMiddleware, "func (m *JWTAuthMiddleware) AuthRequired")
	assertFileLacks(t, root, authnMiddleware, "RequirePermission")
	assertFileLacks(t, root, authnMiddleware, "RoutePermissionChecker")
	assertFileContains(t, root, authzMiddleware, "type RoutePermissionChecker interface")
	assertFileContains(t, root, authzMiddleware, "RequirePermissionOrGlobal")
}

func TestAuthzBootstrapSeedsUseFourSegmentResourceKeys(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, rel := range []string{
		"configs/mysql/bootstrap.sql",
		"internal/pkg/migration/migrations/000005_bootstrap_system_data.up.sql",
	} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		sql := string(data)
		assertFourSegmentResourceValues(t, rel+" authz_resources.key", extractAuthzResourceKeysFromSQL(t, sql))
	}
}

func TestAuthzAuthorizationDoesNotUseRoleNameAdministratorBypasses(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, check := range []struct {
		path   string
		legacy []string
	}{
		{
			path:   "internal/pkg/middleware/authn/jwt_middleware.go",
			legacy: []string{"RequirePlatformAdmin", "RequirePermissionOrPlatformAdmin", "IsPlatformAdminRole"},
		},
		{
			path:   "internal/apiserver/infra/suggest/access/authz_facts_reader.go",
			legacy: []string{"IsSuperAdmin", "tenant_admin", "super_admin", "IsPlatformAdminRole"},
		},
	} {
		for _, value := range check.legacy {
			assertFileLacks(t, root, check.path, value)
		}
	}
	assertFileContains(t, root, "internal/pkg/middleware/authz/middleware.go", "RequirePermissionOrGlobal")
	assertFileLacks(t, root, "internal/pkg/middleware/authn/jwt_middleware.go", "RequirePermission")
}

func TestAuthzStandaloneBootstrapUsesCanonicalTenantDomain(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	const rel = "configs/mysql/bootstrap.sql"
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	sql := string(data)
	for _, table := range []string{
		"authz_roles",
		"authz_assignments",
		"authz_role_inheritances",
		"authz_permission_grants",
		"authz_policy_versions",
	} {
		statement := extractInsertStatement(t, sql, table)
		if strings.Contains(statement, "'1'") {
			t.Fatalf("%s INSERT for %s contains legacy AuthZ tenant domain '1'; standalone bootstrap must use fangcun", rel, table)
		}
		if !strings.Contains(statement, "'fangcun'") {
			t.Fatalf("%s INSERT for %s does not contain canonical AuthZ tenant domain 'fangcun'", rel, table)
		}
	}
}

func TestAuthzProductionCodeUsesSemanticDomainPackages(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	retiredFacade := modulePath + "internal/apiserver/domain/authz"
	for _, relRoot := range []string{
		"internal/apiserver",
	} {
		scanImports(t, filepath.Join(root, filepath.FromSlash(relRoot)), func(path string, imports []string) {
			rel := filepath.ToSlash(mustRel(t, root, path))
			for _, imp := range imports {
				if imp == retiredFacade {
					t.Fatalf("%s imports root authz facade; production code must use semantic child packages", rel)
				}
			}
		})
	}
}

func TestAuthzTestsDoNotAddRootDomainFacadeImports(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	retiredFacade := modulePath + "internal/apiserver/domain/authz"
	scanImportsIncludingTests(t, filepath.Join(root, "internal", "apiserver"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		if !strings.HasSuffix(rel, "_test.go") {
			return
		}
		for _, imp := range imports {
			if imp != retiredFacade {
				continue
			}
			t.Fatalf("%s imports retired root authz facade; tests must use semantic child packages", rel)
		}
	})
}

func TestAuthzDecisionPolicyLivesInTheAuthorizationDomain(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	assertFileContains(t, root, "internal/apiserver/domain/authz/authorization/evaluator.go", "type Evaluator struct")
	assertFileContains(t, root, "internal/apiserver/domain/authz/authorization/role_resolver.go", "type RoleResolver interface")
	assertFileContains(t, root, "internal/apiserver/infra/authz/runtime/runtime.go", "r.evaluator.Evaluate")
	assertFileContains(t, root, "internal/apiserver/infra/authz/runtime/role_graph.go", "var _ authorization.RoleResolver")
	assertFileLacks(t, root, "internal/apiserver/infra/authz/runtime/snapshot.go", "func (s *Snapshot) Check")
	assertFileLacks(t, root, "internal/apiserver/infra/authz/runtime/snapshot.go", "default-role-manager")
	assertFileLacks(t, root, "internal/apiserver/application/authz/authorization/decision_service.go", "NativeChecker")

	retiredRuntime := filepath.Join(root, "internal", "apiserver", "domain", "authz", "runtime")
	matches, err := filepath.Glob(filepath.Join(retiredRuntime, "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) > 0 {
		t.Fatalf("retired AuthZ domain runtime package still contains files: %v", matches)
	}
}

func TestAuthzDomainDoesNotImportApplication(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	scanImports(t, filepath.Join(root, "internal", "apiserver", "domain", "authz"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, imp := range imports {
			if strings.HasPrefix(imp, modulePath+"internal/apiserver/application/authz") {
				t.Fatalf("%s imports %s; authz domain must not depend on application", rel, imp)
			}
		}
	})
}

func TestAuthzCrossAggregatePolicyNotInApplication(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	dependencyPath := filepath.Join(root, "internal", "apiserver", "application", "authz", "policychange", "dependency_validation.go")
	if _, err := os.Stat(dependencyPath); err == nil {
		t.Fatalf("dependency_validation.go must be removed from application layer")
	}
	assertFileContains(t, root, "internal/apiserver/domain/authz/policy/resource_change_policy.go", "ResourceChangePolicy")
	assertFileContains(t, root, "internal/apiserver/domain/authz/policy/role_removal_policy.go", "RoleRemovalPolicy")
}

func TestAuthzGRPCAssignmentAdmissionNeverFailOpen(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	assertFileContains(t, root, "internal/apiserver/container/authz/module.go", "assignment constraints file is required")
	assertFileContains(t, root, "internal/apiserver/transport/grpc/service/authz/service.go",
		`recordAssignmentAuthorization(request.CallerService, string(request.Operation), "failed")`)
}

func TestAuthzApplicationDoesNotComputeManagedAssignmentDiff(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	assertFileLacks(t, root, "internal/apiserver/application/authz/assignment/command_service.go", "currentManaged := make(map[string]")
	assertFileContains(t, root, "internal/apiserver/domain/authz/assignment/replacement_policy.go", "type ReplacementPlan struct")
}

func TestSuggestProfileSuggestionBoundaries(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	scanImports(t, filepath.Join(root, "internal", "apiserver", "application", "suggest"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, imp := range imports {
			if strings.HasPrefix(imp, modulePath+"internal/apiserver/infra/") ||
				imp == "os" ||
				imp == "path/filepath" ||
				imp == "github.com/robfig/cron/v3" {
				t.Fatalf("%s imports %s; suggest application must depend on profile candidate ports instead of infra, filesystem, or cron scheduling", rel, imp)
			}
		}
	})

	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "domain", "suggest"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, token := range []string{
			"name|id|mobiles",
			"Trie",
			"Hash",
			"ImportLines",
			"cron",
		} {
			if strings.Contains(source, token) {
				t.Fatalf("%s contains suggest infrastructure language %q; keep the domain to profile candidates, query, and ranking rules", rel, token)
			}
		}
	})

	scanImports(t, filepath.Join(root, "internal", "apiserver", "transport", "rest", "suggest"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, imp := range imports {
			if strings.HasPrefix(imp, modulePath+"internal/apiserver/infra/suggest") {
				t.Fatalf("%s imports %s; REST suggest transport must depend on application ProfileSuggestor", rel, imp)
			}
		}
	})

	assertFileLacks(t, root, "internal/apiserver/transport/rest/suggest/handler.go", "AllowMobileSearch")
	assertFileContains(t, root, "internal/apiserver/domain/suggest/search_policy.go", "type SearchPolicy struct")
	assertFileContains(t, root, "internal/apiserver/domain/suggest/scope_resolution.go", "type ScopeResolutionPolicy struct")
	assertFileContains(t, root, "internal/apiserver/domain/suggest/index_mutation.go", "type ProfileIndexMutation struct")
	assertFileContains(t, root, "internal/apiserver/application/suggest/service.go", "searchPolicy")
}

func TestSuggestRetiredNumericTenantFieldsDoNotReturn(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, rel := range []string{
		"internal/apiserver/domain/suggest/principal.go",
		"internal/apiserver/domain/suggest/profile.go",
		"internal/apiserver/domain/suggest/scope.go",
		"internal/apiserver/infra/mysql/suggest/loader.go",
	} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range []string{"TenantID int64", "TenantIDs", "tenant_id"} {
			if strings.Contains(string(data), token) {
				t.Fatalf("%s contains retired numeric suggest tenant field %q; use TenantDomain for authorization and OrgIDs for data visibility", rel, token)
			}
		}
	}
}

func TestDataAccessPackagesDoNotDependOnTransportImplementations(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, relRoot := range []string{
		"internal/apiserver/infra/mysql",
		"internal/pkg/database",
		"internal/pkg/migration",
	} {
		scanImports(t, filepath.Join(root, relRoot), func(path string, imports []string) {
			rel := filepath.ToSlash(mustRel(t, root, path))
			for _, imp := range imports {
				if strings.HasPrefix(imp, modulePath+"internal/apiserver/interface/") ||
					strings.HasPrefix(imp, modulePath+"internal/apiserver/transport/") {
					t.Fatalf("%s imports %s; data access packages must not depend on transport implementations", rel, imp)
				}
			}
		})
	}
}

func TestAPIServerCompositionSettersAreAllowlisted(t *testing.T) {
	t.Parallel()

	scanContainerModuleGoFiles(t, func(path string, file *ast.File) {
		root := repoRoot(t)
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || !strings.HasPrefix(fn.Name.Name, "Set") {
				continue
			}
			t.Fatalf("%s:%s.%s is a new composition setter; add a module graph/post-wire seam before allowing it", rel, receiverTypeName(fn), fn.Name.Name)
		}
	})
}

func TestGRPCContractsHaveRuntimeAndSDKCompileGuards(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	contracts := []struct {
		module, version, proto, alias, generatedPackage, serviceFile, registerToken, sdkFile string
	}{
		{"authn", "v2", "api/grpc/iam/authn/v2/authn.proto", "authnv2", "api/grpc/iam/authn/v2", "internal/apiserver/transport/grpc/service/authn/service.go", "authnv2.RegisterAuthServiceServer", "pkg/sdk/auth/client/client.go"},
		{"authz", "v3", "api/grpc/iam/authz/v3/authz.proto", "authzv3", "api/grpc/iam/authz/v3", "internal/apiserver/transport/grpc/service/authz/service.go", "authzv3.RegisterAuthorizationServiceServer", "pkg/sdk/authz/client.go"},
		{"identity", "v2", "api/grpc/iam/identity/v2/identity.proto", "identityv2", "api/grpc/iam/identity/v2", "internal/apiserver/transport/grpc/service/identity/service.go", "identityv2.RegisterIdentityReadServer", "pkg/sdk/identity/client.go"},
		{"idp", "v2", "api/grpc/iam/idp/v2/idp.proto", "idpv2", "api/grpc/iam/idp/v2", "internal/apiserver/transport/grpc/service/idp/service.go", "idpv2.RegisterIDPServiceServer", "pkg/sdk/idp/client.go"},
	}
	for _, contract := range contracts {
		assertFileContains(t, root, contract.proto, "package iam."+contract.module+"."+contract.version+";")
		assertFileContains(t, root, contract.proto, "github.com/FangcunMount/iam/v3/"+contract.generatedPackage+";"+contract.alias)
		assertFileContains(t, root, filepath.ToSlash(filepath.Join(contract.generatedPackage, contract.module+".pb.go")), "package "+contract.alias)
		assertFileContains(t, root, filepath.ToSlash(filepath.Join(contract.generatedPackage, contract.module+"_grpc.pb.go")), "package "+contract.alias)
		assertFileContains(t, root, contract.serviceFile, "api/grpc/iam/"+contract.module+"/"+contract.version)
		assertFileContains(t, root, contract.serviceFile, contract.registerToken)
		assertFileContains(t, root, contract.sdkFile, "api/grpc/iam/"+contract.module+"/"+contract.version)
	}
	assertFileContains(t, root, "pkg/sdk/public_api_compile_test.go", `github.com/FangcunMount/iam/v3/pkg/sdk`)

	grpcRoot := filepath.Join(root, "api", "grpc", "iam")
	err := filepath.WalkDir(grpcRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".proto") {
			return nil
		}
		rel := filepath.ToSlash(mustRel(t, root, path))
		if strings.Contains(rel, "/v1/") {
			t.Fatalf("%s is a retired gRPC v1 contract", rel)
		}
		if strings.Contains(rel, "/authz/") && !strings.Contains(rel, "/authz/v3/") {
			t.Fatalf("%s is not the required AuthZ v3 contract", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGoModuleV3BoundaryAndRetiredSDKSymbolsDoNotRegress(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	assertFileContains(t, root, "go.mod", "module github.com/FangcunMount/iam/v3")
	assertFileLacks(t, root, "pkg/sdk/auth/verifier/types.go", "TenantID string")
	assertFileLacks(t, root, "pkg/sdk/auth/jwks/types.go", "type JWKSStats struct")
	assertFileLacks(t, root, "pkg/sdk/public_api_compile_test.go", "authjwks.JWKSStats")
	assertFileLacks(t, root, "pkg/sdk/public_api_compile_test.go", "claims.TenantID")
	assertFileContains(t, root, "internal/apiserver/docs/swagger.yaml", "github_com_FangcunMount_iam_v2_")
	assertFileLacks(t, root, "internal/apiserver/docs/swagger.yaml", "github_com_FangcunMount_iam_v3_")

	retiredModulePath := "github.com/FangcunMount/iam/" + "v2/"
	scanImportsIncludingTests(t, root, func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, imp := range imports {
			if strings.HasPrefix(imp, retiredModulePath) {
				t.Fatalf("%s imports retired Go module path %s; v3 code must import github.com/FangcunMount/iam/v3", rel, imp)
			}
		}
	})
}

func TestRetiredInternalJWKSHelpersDoNotRegress(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, token := range []string{
		"GetJWKSStats",
		"type JWKSStats struct",
		"GetCacheControl",
		"ValidateJWKS",
	} {
		assertFileLacks(t, root, "internal/apiserver/infra/token/keyset/keyset_builder.go", token)
	}
}

func TestDurableOutboxEventsAreNotDirectPublishedToMQ(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	cfg, err := eventcatalog.Load(filepath.Join(root, "configs", "events.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	durableTopics := map[string]struct{}{}
	for eventType, eventCfg := range cfg.Events {
		if eventCfg.Delivery != eventcatalog.DeliveryClassDurableOutbox {
			continue
		}
		topicName, ok := cfg.GetTopicName(eventType)
		if !ok {
			t.Fatalf("durable event %s has no topic name", eventType)
		}
		durableTopics[topicName] = struct{}{}
	}
	if len(durableTopics) == 0 {
		t.Fatal("event catalog has no durable_outbox events; ratchet would not protect durable publishing")
	}

	scanGoSources(t, filepath.Join(root, "internal", "apiserver"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		if rel == "internal/apiserver/infra/messaging/outbox_relay.go" {
			return
		}
		if !strings.Contains(source, "PublishMessage") && !strings.Contains(source, ".Publish(") {
			return
		}
		for topic := range durableTopics {
			if strings.Contains(source, topic) {
				t.Fatalf("%s directly publishes durable_outbox topic %q; stage it in outbox and let the relay publish", rel, topic)
			}
		}
	})
}

func TestApplicationTransactionCallbacksUseTransactionContext(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	outerCtxTxCall := regexp.MustCompile(`\b(?:tx(?:\.[A-Za-z0-9_]+)+|tx[A-Za-z0-9_]*\.[A-Za-z0-9_]+|editor\.[A-Za-z0-9_]+|statusManager\.[A-Za-z0-9_]+)\(ctx\b`)
	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "application"), func(path, source string) {
		if !strings.Contains(source, "WithinTx(ctx, func(txCtx") {
			return
		}
		if match := outerCtxTxCall.FindString(source); match != "" {
			rel := filepath.ToSlash(mustRel(t, root, path))
			t.Fatalf("%s uses outer ctx in transaction callback call %q; use txCtx for tx-scoped repositories and domain collaborators", rel, match)
		}
	})
}

func TestIdentityLegacyChildGuardianshipModelDoesNotReturn(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	roots := []string{
		"api/grpc/iam/identity/v2",
		"api/rest/identity.v2.yaml",
		"configs/grpc_acl.yaml",
		"configs/mysql",
		"internal/apiserver/application/identity",
		"internal/apiserver/domain/identity",
		"internal/apiserver/infra/mysql",
		"internal/apiserver/transport/grpc/service/identity",
		"internal/apiserver/transport/rest/identity",
		"internal/pkg/migration/migrations",
		"pkg/sdk/identity",
	}
	for _, relRoot := range roots {
		path := filepath.Join(root, filepath.FromSlash(relRoot))
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() {
			assertNoUCLegacyToken(t, root, path)
			continue
		}
		err = filepath.WalkDir(path, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			assertNoUCLegacyToken(t, root, path)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestIdentitySemanticServiceNamesDoNotRegress(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, relRoot := range []string{
		"internal/apiserver/application/identity",
		"internal/apiserver/domain/identity",
	} {
		scanGoSources(t, filepath.Join(root, filepath.FromSlash(relRoot)), func(path, source string) {
			rel := filepath.ToSlash(mustRel(t, root, path))
			for _, token := range []string{
				"ApplicationService",
				"NewProfileService",
				"NewManagerService",
				"ProfileLinkManager",
				"Registrar",
				"NewRegistrar",
				"RegisterUserDTO",
				"RegisterProfileDTO",
				"ValidateRegister",
				"ActionRegister",
			} {
				if strings.Contains(source, token) {
					t.Fatalf("%s contains retired identity service name %q; use semantic capabilities such as Creator, Editor, Directory, MyProfiles, Linker, or Commands", rel, token)
				}
			}
		})

		scanGoFiles(t, filepath.Join(root, filepath.FromSlash(relRoot)), func(path string, file *ast.File) {
			rel := filepath.ToSlash(mustRel(t, root, path))
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						typeSpec, ok := spec.(*ast.TypeSpec)
						if !ok {
							continue
						}
						if strings.HasSuffix(typeSpec.Name.Name, "Manager") {
							t.Fatalf("%s declares %s; identity service names should express domain capability rather than Manager", rel, typeSpec.Name.Name)
						}
					}
				case *ast.FuncDecl:
					if strings.HasPrefix(d.Name.Name, "New") && strings.HasSuffix(d.Name.Name, "Manager") {
						t.Fatalf("%s declares %s; identity constructors should express domain capability rather than Manager", rel, d.Name.Name)
					}
				}
			}
		})
	}
}

func TestAuthnOnboardingAndProfileLinkContractsDoNotRegress(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "internal", "apiserver", "application", "authn", "register")); err == nil {
		t.Fatal("internal/apiserver/application/authn/register is retired; use application/authn/signup")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}

	assertFileContains(t, root, "api/grpc/iam/authn/v2/authn.proto", "login_identity_id")

	assertFileContains(t, root, "api/grpc/iam/identity/v2/identity.proto", "rpc EstablishProfileLink")
	assertFileLacks(t, root, "api/grpc/iam/identity/v2/identity.proto", "CreateProfileLink")
	assertFileLacks(t, root, "pkg/sdk/identity/profile_link_command.go", "CreateProfileLink")

	for _, rel := range []string{
		"api/rest/authn.v2.yaml",
		"internal/apiserver/docs/swagger.yaml",
	} {
		assertFileContains(t, root, rel, "/authn/signups/wechat-miniprogram")
	}
	assertFileContains(t, root, "internal/apiserver/transport/rest/authn/router.go", `"/signups"`)
	assertFileContains(t, root, "internal/apiserver/transport/rest/authn/router.go", `"/wechat-miniprogram"`)
	assertFileLacks(t, root, "internal/apiserver/transport/rest/authn/router.go", "/wechat/register")
}

func TestAuthnTokenDomainStaysBehindPortsAndAdapters(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, rel := range []string{
		"internal/apiserver/domain/authn/jwks",
		"internal/apiserver/infra/authentication",
		"internal/apiserver/infra/jwt",
	} {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(rel), "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) > 0 {
			t.Fatalf("%s is retired token implementation code; keep Token domain services in domain/authn/token and JWT encoding in infra/token/jwt", rel)
		}
	}

	forbiddenImports := map[string]struct{}{
		modulePath + "internal/apiserver/domain/authn/jwks":    {},
		modulePath + "internal/apiserver/infra/authentication": {},
		modulePath + "internal/apiserver/infra/jwt":            {},
		"github.com/golang-jwt/jwt/v4":                         {},
		"github.com/golang-jwt/jwt/v5":                         {},
		"github.com/golang-jwt/jwt":                            {},
	}
	scanImportsIncludingTests(t, filepath.Join(root, "internal"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		if strings.HasPrefix(rel, "internal/apiserver/infra/token/jwt/") ||
			strings.HasPrefix(rel, "internal/pkg/architecture/") {
			return
		}
		for _, imp := range imports {
			if _, forbidden := forbiddenImports[imp]; forbidden {
				t.Fatalf("%s imports %s; JWT libraries and retired token packages must stay behind infra/token/jwt or domain token ports", rel, imp)
			}
		}
	})

	scanImportsIncludingTests(t, filepath.Join(root, "internal", "apiserver", "application", "authn"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, imp := range imports {
			if strings.HasPrefix(imp, modulePath+"internal/apiserver/infra/token/") ||
				imp == "crypto/rsa" ||
				strings.HasPrefix(imp, "github.com/golang-jwt/") {
				t.Fatalf("%s imports %s; application/authn must depend on token ports and DTOs, not JWT/key infrastructure", rel, imp)
			}
		}
	})

	scanImportsIncludingTests(t, filepath.Join(root, "internal", "apiserver", "infra", "token", "jwt"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, imp := range imports {
			if strings.HasPrefix(imp, modulePath+"internal/apiserver/application/authn/") {
				t.Fatalf("%s imports %s; JWT infrastructure must implement domain token ports without depending on application packages", rel, imp)
			}
		}
	})

	for _, rel := range []string{
		"internal/apiserver/application/authn/token/issuer.go",
		"internal/apiserver/application/authn/token/refresher.go",
		"internal/apiserver/application/authn/token/verifier.go",
		"internal/apiserver/application/authn/token/revoker.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
			t.Fatalf("%s duplicates Token lifecycle behavior owned by domain/authn/token", rel)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}

	forbiddenDomainTokens := []string{
		"AccessClaims",
		"AuthJWT" + "Token",
		"AMRJWT" + "Token",
		"AuthBearer" + "Token",
		"AMRBearer" + "Token",
		"Bearer" + "TokenCredential",
		"Bearer" + "TokenAuthStrategy",
		"Token" + "Verifier",
		"Register" + "CredentialBuilder",
		"Credential" + "Builder",
		"credential" + "Builders",
		"create" + "Strategy",
		"type " + "AuthInput struct",
		"Authenticater",
		"JWT" + "TokenCredential",
		"JWT" + "TokenAuthStrategy",
		"FlattenClaimsFor" + "JWT",
		"JWK",
		"JWKS",
		"RS256",
		"PEM",
		"SigningMethod",
		"RegisteredClaims",
		"crypto/rsa",
		"type Scenario string",
		"AuthPassword Scenario",
		"AuthPhoneOTP Scenario",
		"AuthWxMinip  Scenario",
		"AuthWecom    Scenario",
		".Scenario()",
	}
	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "domain", "authn"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, token := range forbiddenDomainTokens {
			if strings.Contains(source, token) {
				t.Fatalf("%s contains retired JWT-shaped domain token %q; keep JWT claims and strategies in infra/token/jwt", rel, token)
			}
		}
	})

	scanGoSources(t, filepath.Join(root, "internal", "apiserver"), func(path, source string) {
		if !strings.Contains(source, "jwt_token") {
			return
		}
		rel := filepath.ToSlash(mustRel(t, root, path))
		if strings.HasPrefix(rel, "internal/apiserver/transport/rest/authn/") ||
			strings.HasPrefix(rel, "internal/apiserver/transport/grpc/service/authn/") {
			return
		}
		t.Fatalf("%s contains jwt_token compatibility wording; only transport authn adapters may understand this wire value", rel)
	})

	forbiddenAuthnDocTokens := []string{
		"AuthInput",
		"issue" + "JWT",
		"JWT Token 认证",
		"Authenticator Service",
		"策略工厂",
		"strategy" + "Factory",
		"Strategy" + "Factory",
	}
	for _, rel := range []string{
		"internal/apiserver/domain/authn/authentication/doc.go",
		"internal/apiserver/application/authn/README.md",
	} {
		authnDocBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range forbiddenAuthnDocTokens {
			if strings.Contains(string(authnDocBytes), token) {
				t.Fatalf("%s contains retired authn wording %q", rel, token)
			}
		}
	}

	forbiddenApplicationJWKSLifecycleTokens := []string{
		"func (k *ManagedKey) Enter" + "Grace",
		"func (k *ManagedKey) Retire",
		"func (k *ManagedKey) Force" + "Retire",
		"func (k *ManagedKey) Can" + "Sign",
		"func (k *ManagedKey) Can" + "Verify",
		"func (k *ManagedKey) Should" + "Publish",
		"func (k *ManagedKey) Is" + "Expired",
		"func (k *ManagedKey) Is" + "NotYetValid",
		"func (k *ManagedKey) Is" + "ValidAt",
	}
	appJWKSModelsBytes, err := os.ReadFile(filepath.Join(root, "internal", "apiserver", "application", "authn", "jwks", "models.go"))
	if err != nil {
		t.Fatal(err)
	}
	appJWKSModels := string(appJWKSModelsBytes)
	for _, token := range forbiddenApplicationJWKSLifecycleTokens {
		if strings.Contains(appJWKSModels, token) {
			t.Fatalf("application/authn/jwks/models.go contains signing key lifecycle behavior %q; keep key lifecycle behavior in infra/token/keyset", token)
		}
	}

	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "infra", "token", "keyset"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		if strings.Contains(source, "type Key =") || strings.Contains(source, "type KeyStatus =") {
			t.Fatalf("%s aliases application jwks lifecycle types; keyset must own signing key models and map to application DTOs", rel)
		}
	})
}

func TestJWKSMutationsUseSingleApplicationLifecycleBoundary(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	appDir := filepath.Join(root, "internal", "apiserver", "application", "authn", "jwks")
	for _, retired := range []string{
		"KeyRotationAppService",
		"KeyRotatorPort",
		"Should" + "Rotate",
		"Get" + "RotationPolicy",
		"Update" + "RotationPolicy",
		"Get" + "RotationStatus",
		"type Rotation" + "Policy struct",
		"type Key" + "Status ",
	} {
		scanGoSources(t, appDir, func(path, source string) {
			if strings.Contains(source, retired) {
				rel := filepath.ToSlash(mustRel(t, root, path))
				t.Fatalf("%s contains retired JWKS application surface %q", rel, retired)
			}
		})
	}

	for _, rel := range []string{
		"internal/apiserver/transport/rest/authn",
		"internal/apiserver/infra/scheduler",
	} {
		scanImports(t, filepath.Join(root, filepath.FromSlash(rel)), func(path string, imports []string) {
			for _, imp := range imports {
				if imp == modulePath+"internal/apiserver/infra/token/keyset" {
					t.Fatalf("%s imports keyset directly; JWKS mutations must pass through KeyLifecycleAppService", filepath.ToSlash(mustRel(t, root, path)))
				}
			}
		})
	}

	readSource := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	handler := readSource("internal/apiserver/transport/rest/authn/handler/jwks_admin_lifecycle.go")
	for _, retired := range []string{
		"keyManagementApp.RetireKey",
		"keyManagementApp.ForceRetireKey",
		"keyManagementApp.EnterGracePeriod",
		"keyManagementApp.CleanupExpiredKeys",
	} {
		if strings.Contains(handler, retired) {
			t.Fatalf("JWKS admin lifecycle handler bypasses KeyLifecycleAppService with %q", retired)
		}
	}
	for _, expected := range []string{
		"keyLifecycleApp.RetireKey",
		"keyLifecycleApp.ForceRetireKey",
		"keyLifecycleApp.EnterGracePeriod",
		"keyLifecycleApp.CleanupExpiredKeys",
	} {
		if !strings.Contains(handler, expected) {
			t.Fatalf("JWKS admin lifecycle handler is missing %q", expected)
		}
	}

	createHandler := readSource("internal/apiserver/transport/rest/authn/handler/jwks_admin_keys.go")
	if !strings.Contains(createHandler, "keyLifecycleApp.CreateAndActivate") {
		t.Fatal("JWKS admin create does not use KeyLifecycleAppService")
	}
	scheduler := readSource("internal/apiserver/container/authn/scheduler.go")
	if !strings.Contains(scheduler, "m.keyLifecycleApp") {
		t.Fatal("JWKS scheduler is not wired to the shared KeyLifecycleAppService")
	}
	rest := readSource("internal/apiserver/container/authn/rest.go")
	if !strings.Contains(rest, "caps.KeyLifecycleApp") {
		t.Fatal("JWKS REST collector is not wired to KeyLifecycleAppService")
	}
}

func TestAuthnLoginMethodSelectionUsesMethodRegistryAndProofFactory(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "internal", "apiserver", "application", "authn", "login", "method_authenticator.go")); err == nil {
		t.Fatal("application/authn/session/method_authenticator.go is retired; sign-in methods prepare domain credentials and domain Authenticator dispatches normal authentication")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"internal/apiserver/application/authn/session/scenario_selector.go",
		"internal/apiserver/application/authn/session/scenario_selector_explicit.go",
		"internal/apiserver/application/authn/session/scenario_selector_legacy.go",
		"internal/apiserver/application/authn/session/method_catalog.go",
		"internal/apiserver/application/authn/session/method_password.go",
		"internal/apiserver/application/authn/session/method_phone_otp.go",
		"internal/apiserver/application/authn/session/method_wechat.go",
		"internal/apiserver/application/authn/session/method_wecom.go",
		"internal/apiserver/application/authn/session/method_bearer.go",
		"internal/apiserver/application/authn/session/method_authenticator_test.go",
		"internal/apiserver/application/authn/session/signin_method_catalog_test.go",
		"internal/apiserver/application/authn/session/method_proof_preparer_test.go",
		"internal/apiserver/application/authn/session/adapter_bearer.go",
		"internal/apiserver/application/authn/session/adapter_catalog.go",
		"internal/apiserver/application/authn/session/adapter_password.go",
		"internal/apiserver/application/authn/session/adapter_phone_otp.go",
		"internal/apiserver/application/authn/session/adapter_wechat_mini.go",
		"internal/apiserver/application/authn/session/adapter_wecom.go",
		"internal/apiserver/application/authn/session/explicit_payload_adapter.go",
		"internal/apiserver/application/authn/session/method_selector.go",
		"internal/apiserver/application/authn/session/method_selector_explicit.go",
		"internal/apiserver/application/authn/session/method_selector_legacy.go",
		"internal/apiserver/application/authn/session/services.go",
		"internal/apiserver/application/authn/session/services_impl.go",
		"internal/apiserver/application/authn/session/method/bearer.go",
		"internal/apiserver/application/authn/session/proof/bearer.go",
		"internal/apiserver/application/authn/session/compatibility/bearer_strategy.go",
		"internal/apiserver/application/authn/session/compatibility/bearer_strategy_test.go",
		"internal/apiserver/application/authn/session/signin/sign_in.go",
		"internal/apiserver/application/authn/session/signin/deps.go",
		"internal/apiserver/application/authn/session/signin/method/selector.go",
		"internal/apiserver/application/authn/session/signin/proof/factory.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
			t.Fatalf("%s is retired login wording; use method registry, proof factory, and SignIn/SignOut use cases", rel)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}

	assertFileContains(t, root, "internal/apiserver/application/authn/signin/method/selector.go", "type Registry struct")
	assertFileContains(t, root, "internal/apiserver/application/authn/signin/method/types.go", "type LoginMethod interface")
	assertFileContains(t, root, "internal/apiserver/application/authn/signin/method/types.go", "type LoginRequest struct")
	assertFileContains(t, root, "internal/apiserver/application/authn/signin/method/types.go", "type LoginMethodSelection struct")
	assertFileContains(t, root, "internal/apiserver/application/authn/signin/method/payload.go", "type Payload interface")
	assertFileContains(t, root, "internal/apiserver/application/authn/signin/method/payload.go", "func CommonPayloadFromLoginRequest")
	assertFileContains(t, root, "internal/apiserver/application/authn/signin/method/types.go", "CommonPayload")
	assertFileContains(t, root, "internal/apiserver/application/authn/signin/compatibility/explicit_payload.go", "BuildExplicitWireLoginRequest")
	assertFileContains(t, root, "internal/apiserver/application/authn/signin/method/selector.go", "func (s *Registry) Select")
	assertFileContains(t, root, "internal/apiserver/application/authn/signin/proof/factory.go", "type Factory struct")
	assertFileContains(t, root, "internal/apiserver/application/authn/session/service.go", "type Dependencies struct")
	assertFileContains(t, root, "internal/apiserver/application/authn/session/service.go", "RenewSession(ctx context.Context, refreshToken string)")
	assertFileLacks(t, root, "internal/apiserver/application/authn/session/service.go", "Reauthenticate(ctx context.Context")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/payload.go", "Common() CommonPayload")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/password.go", "CommonPayload")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/phone_otp.go", "CommonPayload")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/wechat_mini.go", "CommonPayload")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/wechat_scan.go", "CommonPayload")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/wecom.go", "CommonPayload")
	assertFileLacks(t, root, "internal/apiserver/application/authn/session/service.go", "method.DefaultSelector")
	assertFileLacks(t, root, "internal/apiserver/application/authn/session/service.go", "proof.DefaultFactory")
	assertFileLacks(t, root, "internal/apiserver/application/authn/session/service.go", "NewBearerTokenAuthStrategy")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/sign_in.go", "DefaultSelector")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/sign_in.go", "DefaultFactory")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/selector.go", "type AuthType")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/selector.go", "type Kind")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/selector.go", "type Definition interface")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/selector.go", "type LoginMethod interface")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/selector.go", "SelectionMode")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/types.go", "type Payload interface")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/types.go", "type Attempt struct")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/types.go", "type Command struct")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/types.go", "LoginMethodCommand")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/compatibility/explicit_payload.go", "BuildExplicitLoginMethodCommand")
	assertFileLacks(t, root, "internal/apiserver/application/authn/session/types.go", "SignInAttempt")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/types.go", "AuthMethodJWTToken")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/types.go", "CredentialKindAccessToken")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/method/selector.go", "NewBearer")
	assertFileLacks(t, root, "internal/apiserver/application/authn/signin/proof/factory.go", "NewBearerBuilder")

	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "application", "authn", "session"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, token := range []string{
			"type MethodAuthenticator interface",
			"newMethodAuthenticatorRouter",
			"map[MethodKind]MethodAuthenticator",
			"domainMethodAuthenticator",
			"type MethodKind",
			"MethodBearerToken",
			"type ScenarioSelector interface",
			"type SelectedMethod struct",
			"type CredentialPreparer interface",
			"PrepareCredential(",
			"type SignInMethodDefinition",
			"MethodRoute",
			"newDefaultSignInMethodCatalog",
			"newSignInMethodCatalog",
			"mustSignInMethodCatalog",
			"signInMethodDeps",
			"ScenarioSelection",
			"buildPasswordProof",
			"buildPhoneOTPProof",
			"type signInAdapter interface",
			"newDefaultSignInAdapterCatalog",
			"newSignInAdapterCatalog",
			"mustSignInAdapterCatalog",
			"BearerCompatibilityAdapter",
			"DomainProofAdapter",
			"PrepareProof(",
			"legacyPasswordPayload",
			"legacyPhoneOTPPayload",
		} {
			if strings.Contains(source, token) {
				t.Fatalf("%s contains retired login method router token %q; keep request adaptation in MethodRegistry, proof construction in ProofFactory, and normal authentication in domain Authenticator", rel, token)
			}
		}
	})
	assertFileLacks(t, root, "internal/apiserver/transport/rest/authn/handler/auth_login.go", "ScenarioSelection")
}

func TestAuthnChallengeScenesStayBehindChallengeUseCases(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	challengeImport := modulePath + "internal/apiserver/application/authn/challenge"

	scanImportsIncludingTests(t, filepath.Join(root, "internal", "apiserver", "application", "authn", "linking"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, imp := range imports {
			if imp == challengeImport {
				t.Fatalf("%s imports %s; linking must depend on its local phone-link challenge verifier port", rel, imp)
			}
		}
	})

	scanGoSources(t, filepath.Join(root, "internal", "apiserver"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		if strings.HasPrefix(rel, "internal/apiserver/application/authn/challenge/") ||
			strings.HasPrefix(rel, "internal/pkg/architecture/") {
			return
		}
		for _, token := range []string{"SceneLoginPhoneOTP", "SceneLinkPhoneOTP"} {
			if strings.Contains(source, token) {
				t.Fatalf("%s references %s; challenge scene selection must stay behind explicit challenge use cases", rel, token)
			}
		}
	})
}

func TestAuthnConsumesOnlyExternalIdentityResolverBoundary(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	forbiddenImports := []string{
		modulePath + "internal/apiserver/application/idp/prepare",
		modulePath + "internal/apiserver/domain/idp/wechatapp",
		modulePath + "internal/apiserver/infra/wechat",
		modulePath + "internal/apiserver/infra/wechatapi/port",
	}
	for _, relRoot := range []string{
		"internal/apiserver/application/authn",
		"internal/apiserver/domain/authn",
		"internal/apiserver/container/authn",
	} {
		scanImportsIncludingTests(t, filepath.Join(root, filepath.FromSlash(relRoot)), func(path string, imports []string) {
			rel := filepath.ToSlash(mustRel(t, root, path))
			for _, imp := range imports {
				for _, forbidden := range forbiddenImports {
					if imp == forbidden || strings.HasPrefix(imp, forbidden+"/") {
						t.Fatalf("%s imports %s; AuthN must consume IDP only through ExternalIdentity Resolver", rel, imp)
					}
				}
			}
		})
		scanGoSources(t, filepath.Join(root, filepath.FromSlash(relRoot)), func(path, source string) {
			rel := filepath.ToSlash(mustRel(t, root, path))
			for _, token := range []string{"AppSecret", "CorpSecret", "WecomAgentID", "IdentityProvider"} {
				if strings.Contains(source, token) {
					t.Fatalf("%s contains %q; provider secret and SDK exchange details belong to IDP", rel, token)
				}
			}
		})
	}

	for _, rel := range []string{
		"internal/apiserver/application/idp/prepare/resolve.go",
		"internal/apiserver/application/idp/prepare/doc.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
			t.Fatalf("%s is retired; provider preparation belongs to application/idp/externalidentity", rel)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	assertFileContains(t, root, "internal/apiserver/container/idp/module.go", "ExternalIdentityResolver()")
	assertFileContains(t, root, "internal/apiserver/application/idp/externalidentity/resolver.go", "type Resolver interface")
}

func TestAuthnAndAuthzDomainsUseIdentityUserCapabilitiesInsteadOfRepository(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	forbiddenImports := map[string]struct{}{
		modulePath + "internal/apiserver/domain/identity/user": {},
		modulePath + "internal/apiserver/infra/mysql/user":     {},
	}
	for _, relRoot := range []string{
		"internal/apiserver/domain/authn",
		"internal/apiserver/domain/authz",
	} {
		scanImportsIncludingTests(t, filepath.Join(root, filepath.FromSlash(relRoot)), func(path string, imports []string) {
			rel := filepath.ToSlash(mustRel(t, root, path))
			for _, imp := range imports {
				if _, forbidden := forbiddenImports[imp]; forbidden {
					t.Fatalf("%s imports %s; AuthN/AuthZ domain must consume Identity through useraccess.UserStatusReader/UserResolver", rel, imp)
				}
			}
		})
	}
}

func TestRetiredTransactionalOutboxLegacyCodeDoesNotReturn(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, rel := range []string{
		strings.Join([]string{"internal", "pkg", "database", "tx", "runner.go"}, "/"),
		strings.Join([]string{"internal", "apiserver", "infra", "messaging", "version_notifier.go"}, "/"),
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
			t.Fatalf("%s is retired legacy code; do not reintroduce old fail-open transaction runner or authz version notifier", rel)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}

	forbiddenTokens := []string{
		"Version" + "Notifier",
		"Version" + "ChangeHandler",
		"Authz" + "VersionTopic",
		"Authz" + "VersionChannel",
		"Default" + "DecodeFailureRetryDelay",
		"type " + "Envelope struct",
	}
	scanGoSources(t, filepath.Join(root, "internal"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		if rel == "internal/pkg/architecture/architecture_test.go" {
			return
		}
		for _, token := range forbiddenTokens {
			if strings.Contains(source, token) {
				t.Fatalf("%s contains retired transactional outbox token %q", rel, token)
			}
		}
	})
}

func isDomainForbiddenImport(imp string) bool {
	return imp == "gorm.io/gorm" ||
		strings.HasPrefix(imp, modulePath+"internal/apiserver/infra/") ||
		strings.HasPrefix(imp, modulePath+"internal/pkg/database") ||
		strings.HasPrefix(imp, modulePath+"internal/pkg/migration") ||
		strings.HasPrefix(imp, modulePath+"internal/apiserver/interface/") ||
		strings.HasPrefix(imp, modulePath+"internal/apiserver/transport/")
}

func isApplicationForbiddenImport(imp string) bool {
	return imp == "gorm.io/gorm" ||
		imp == "github.com/FangcunMount/component-base/pkg/messaging" ||
		strings.HasPrefix(imp, modulePath+"internal/apiserver/infra/") ||
		strings.HasPrefix(imp, modulePath+"internal/pkg/database") ||
		strings.HasPrefix(imp, modulePath+"internal/pkg/migration")
}

func isApplicationTestutilPath(rel string) bool {
	return strings.HasPrefix(rel, "internal/apiserver/application/") &&
		strings.Contains(rel, "/testutil/")
}

func isApplicationTestutilAllowedImport(imp string) bool {
	return imp == "gorm.io/gorm" ||
		imp == "gorm.io/gorm/logger" ||
		strings.HasPrefix(imp, "gorm.io/driver/") ||
		strings.HasPrefix(imp, modulePath+"internal/apiserver/infra/mysql/")
}

func assertAllowlistReasons(t *testing.T, allowlist map[string]string) {
	t.Helper()
	for key, reason := range allowlist {
		if strings.TrimSpace(reason) == "" {
			t.Fatalf("%s has an empty architecture allowlist reason", key)
		}
	}
}

func assertAllowlistStillUsed(t *testing.T, allowlist map[string]string, seen map[string]struct{}) {
	t.Helper()
	for key := range allowlist {
		if _, ok := seen[key]; !ok {
			t.Fatalf("architecture allowlist entry %s no longer matches current imports; remove it", key)
		}
	}
}

func assertNoRetiredArchitectureExceptionReasons(t *testing.T, allowlist map[string]string) {
	t.Helper()
	for key, reason := range allowlist {
		for _, parts := range retiredArchitectureExceptionReasonParts {
			retired := strings.Join(parts, "_")
			if strings.Contains(reason, retired) {
				t.Fatalf("architecture allowlist entry %s uses retired reason %q", key, retired)
			}
		}
	}
}

func scanImports(t *testing.T, root string, visit func(path string, imports []string)) {
	t.Helper()
	scanGoFiles(t, root, func(path string, file *ast.File) {
		imports := make([]string, 0, len(file.Imports))
		for _, spec := range file.Imports {
			imports = append(imports, strings.Trim(spec.Path.Value, `"`))
		}
		visit(path, imports)
	})
}

func scanImportsIncludingTests(t *testing.T, root string, visit func(path string, imports []string)) {
	t.Helper()
	scanGoFilesIncludingTests(t, root, func(path string, file *ast.File) {
		imports := make([]string, 0, len(file.Imports))
		for _, spec := range file.Imports {
			imports = append(imports, strings.Trim(spec.Path.Value, `"`))
		}
		visit(path, imports)
	})
}

func importsForFile(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		imports = append(imports, strings.Trim(spec.Path.Value, `"`))
	}
	return imports
}

func scanGoFiles(t *testing.T, root string, visit func(path string, file *ast.File)) {
	t.Helper()
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		visit(path, file)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func scanGoFilesIncludingTests(t *testing.T, root string, visit func(path string, file *ast.File)) {
	t.Helper()
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		visit(path, file)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func scanGoSources(t *testing.T, root string, visit func(path, source string)) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		visit(path, string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertNoUCLegacyToken(t *testing.T, root, path string) {
	t.Helper()
	rel := filepath.ToSlash(mustRel(t, root, path))
	for _, token := range []string{
		"/child/",
		"/guardianship/",
		"children",
		"guardians",
		"Child",
		"Guardianship",
		"profiles/register",
		"/identity/refs",
		"refs/grant",
		"儿童",
		"孩子",
		"监护",
	} {
		if strings.Contains(rel, token) {
			t.Fatalf("%s contains retired identity model token %q", rel, token)
		}
	}
	switch filepath.Ext(path) {
	case ".go", ".proto", ".yaml", ".yml", ".sql":
	default:
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, token := range []string{
		"internal/apiserver/application/identity/child",
		"internal/apiserver/application/identity/guardianship",
		"internal/apiserver/domain/identity/child",
		"internal/apiserver/domain/identity/guardianship",
		"internal/apiserver/infra/mysql/child",
		"internal/apiserver/infra/mysql/guardianship",
		"/identity/children",
		"/identity/guardians",
		"/identity/profiles/register",
		"/identity/refs",
		"/identity/refs/grant",
		"儿童",
		"孩子",
		"监护",
		"Child",
		"Guardianship",
	} {
		if strings.Contains(source, token) {
			t.Fatalf("%s contains retired identity model token %q", rel, token)
		}
	}
}

func assertFileContains(t *testing.T, root, rel, token string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), token) {
		t.Fatalf("%s does not contain required token %q", rel, token)
	}
}

func assertFileLacks(t *testing.T, root, rel, token string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), token) {
		t.Fatalf("%s contains retired token %q", rel, token)
	}
}

func extractAuthzResourceKeysFromSQL(t *testing.T, sql string) []string {
	t.Helper()
	start := strings.Index(sql, "INSERT INTO `authz_resources`")
	if start < 0 {
		t.Fatal("authz_resources bootstrap insert not found")
	}
	block := sql[start:]
	if end := strings.Index(block, "ON DUPLICATE KEY UPDATE"); end >= 0 {
		block = block[:end]
	}
	matches := regexp.MustCompile(`\(\s*\d+\s*,\s*'([^']+)'`).FindAllStringSubmatch(block, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, match[1])
	}
	if len(values) == 0 {
		t.Fatal("authz_resources bootstrap keys not found")
	}
	return values
}

func extractInsertStatement(t *testing.T, sql, table string) string {
	t.Helper()
	prefix := "INSERT INTO `" + table + "`"
	start := strings.Index(sql, prefix)
	if start < 0 {
		t.Fatalf("%s bootstrap insert not found", table)
	}
	statement := sql[start:]
	end := strings.Index(statement, ";")
	if end < 0 {
		t.Fatalf("%s bootstrap insert is not terminated", table)
	}
	return statement[:end+1]
}

func assertFourSegmentResourceValues(t *testing.T, label string, values []string) {
	t.Helper()
	for _, value := range values {
		parts := strings.Split(value, ":")
		if len(parts) != 4 {
			t.Fatalf("%s contains non-four-segment resource %q", label, value)
		}
		for _, part := range parts {
			if strings.TrimSpace(part) == "" {
				t.Fatalf("%s contains empty resource segment in %q", label, value)
			}
		}
	}
}

func receiverTypeName(fn *ast.FuncDecl) string {
	if fn == nil || fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	switch expr := fn.Recv.List[0].Type.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.StarExpr:
		if ident, ok := expr.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func mustRel(t *testing.T, root, path string) string {
	t.Helper()
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatal(err)
	}
	return rel
}
