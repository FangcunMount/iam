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

	"github.com/FangcunMount/iam/v2/pkg/eventcatalog"
)

const modulePath = "github.com/FangcunMount/iam/v2/"

var activeLegacyApplicationInfrastructureImports = map[string]string{}

var activeAuthzRootFacadeTestImports = map[string]string{}

var retiredArchitectureExceptionReasonParts = [][]string{
	{"application", "test", "support"},
	{"legacy", "uow", "factory", "to", "invert", "in", "phase", "3"},
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
		"AuthzModule.RoleBindingHandler",
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

func TestAuthzCasbinFactsStayBehindApplicationPorts(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, rel := range []string{
		"internal/apiserver/domain/authz/assignment",
		"internal/apiserver/application/authz/assignment",
		"internal/apiserver/infra/mysql/assignment",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("%s must not exist; internal authz language should use rolebinding", rel)
		}
	}

	forbiddenDomainTokens := []string{
		"PolicyRule",
		"GroupingRule",
		"CasbinAdapter",
		"type AuthorizationFactStore interface",
		"BuildPolicyRule",
		"SubjectKey(",
		"RoleKey(",
		"scopeMatch",
		"Sub string",
		"Dom string",
		"Obj string",
		"Act string",
	}
	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "domain", "authz"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, token := range forbiddenDomainTokens {
			if strings.Contains(source, token) {
				t.Fatalf("%s contains Casbin technical authorization fact token %q; keep p/g/r tuples in infra/casbin and use domain business models", rel, token)
			}
		}
	})
	forbiddenDomainCrudTokens := []string{
		"type Commander interface",
		"type Queryer interface",
		"Driving Port",
		"type CreateRoleCommand",
		"type UpdateRoleCommand",
		"type CreateResourceCommand",
		"type UpdateResourceCommand",
		"type GrantCommand",
		"type RevokeCommand",
		"type RevokeByIDCommand",
		"type ListBySubjectQuery",
		"type ListByRoleQuery",
	}
	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "domain", "authz"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, token := range forbiddenDomainCrudTokens {
			if strings.Contains(source, token) {
				t.Fatalf("%s contains application driving model %q; keep authz domain to entities, value objects, repositories, and pure domain services", rel, token)
			}
		}
	})

	scanImports(t, filepath.Join(root, "internal", "apiserver", "transport", "grpc", "service", "authz"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, imp := range imports {
			if strings.HasPrefix(imp, modulePath+"internal/apiserver/infra/casbin") ||
				strings.HasPrefix(imp, modulePath+"internal/apiserver/domain/authz/policy") ||
				strings.HasPrefix(imp, modulePath+"internal/apiserver/domain/authz/rolebinding") ||
				strings.HasPrefix(imp, modulePath+"internal/apiserver/domain/authz/role") {
				t.Fatalf("%s imports %s; authz gRPC transport must depend on authorization application use cases", rel, imp)
			}
		}
	})

	scanImports(t, filepath.Join(root, "internal", "apiserver", "transport", "rest", "authz"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, imp := range imports {
			if strings.HasPrefix(imp, modulePath+"internal/apiserver/infra/casbin") {
				t.Fatalf("%s imports %s; REST authz handlers must not directly depend on Casbin infrastructure", rel, imp)
			}
		}
	})

	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "transport"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, token := range []string{".Enforce(", ".GetRolesForUser("} {
			if strings.Contains(source, token) {
				t.Fatalf("%s directly calls %s; transport must use RouteAuthorizationRuntime or application authorization ports", rel, token)
			}
		}
	})

	scanImports(t, filepath.Join(root, "internal", "apiserver", "application", "authz"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, imp := range imports {
			if strings.HasPrefix(imp, modulePath+"internal/apiserver/infra/casbin") {
				t.Fatalf("%s imports %s; authz application must depend on business ports, not Casbin infrastructure", rel, imp)
			}
		}
	})

	assertFileContains(t, root, "configs/casbin_model.conf", "r = sub, dom, obj, act, scope")
	assertFileContains(t, root, "configs/casbin_model.conf", "p = sub, dom, obj, act, scope")
	assertFileContains(t, root, "configs/casbin_model.conf", "resourceMatch(r.obj, p.obj)")
	assertFileContains(t, root, "configs/casbin_model.conf", "actionMatch(r.act, p.act)")
	assertFileContains(t, root, "configs/casbin_model.conf", "scopeMatch(r.scope, p.scope)")
	assertFileLacks(t, root, "configs/casbin_model.conf", "keyMatch(r.obj, p.obj)")
	assertFileLacks(t, root, "configs/casbin_model.conf", "regexMatch(r.act, p.act)")
	assertFileContains(t, root, "api/grpc/iam/authz/v2/authz.proto", "scope_type")
	assertFileContains(t, root, "api/grpc/iam/authz/v2/authz.proto", "scope_value")
	assertFileContains(t, root, "internal/apiserver/transport/rest/authz/dto/policy.go", "ScopeType")
	assertFileContains(t, root, "internal/apiserver/transport/rest/authz/dto/check.go", "ScopeType")
	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "transport", "rest", "authz"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, token := range []string{
			"policyDomain.PolicyRule",
			"policyDomain.GroupingRule",
			"policy.CasbinAdapter",
			"policy.AuthorizationFactStore",
			".AddPolicy(",
			".AddGroupingPolicy(",
		} {
			if strings.Contains(source, token) {
				t.Fatalf("%s contains Casbin technical authz dependency %q; REST authz must consume application authorization use cases", rel, token)
			}
		}
	})

	scanImports(t, filepath.Join(root, "internal", "apiserver", "infra", "mysql", "casbinrule"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, imp := range imports {
			if strings.HasPrefix(imp, modulePath+"internal/apiserver/domain/authz/policy") {
				t.Fatalf("%s imports %s; casbinrule storage must receive business facts through the authz application rule-store port", rel, imp)
			}
		}
	})

	assertFileLacks(t, root, "internal/apiserver/application/authz/uow/uow.go", "policyDomain.AuthorizationFactStore")
	assertFileLacks(t, root, "internal/apiserver/application/authz/uow/uow.go", "RuleStore")
	assertFileLacks(t, root, "internal/pkg/middleware/authn/jwt_middleware.go", "CasbinEnforcer")
	assertFileLacks(t, root, "internal/apiserver/application/authz/policy/command_service.go", "BuildPolicyRule")
	assertFileLacks(t, root, "internal/apiserver/application/authz/policy/command_service.go", ".AddPolicy(")
	assertFileLacks(t, root, "internal/apiserver/application/authz/rolebinding/command_service.go", ".AddGroupingPolicy(")
	assertFileLacks(t, root, "internal/apiserver/container/authz/capabilities.go", "policyDomain.CasbinAdapter")
	assertFileLacks(t, root, "internal/apiserver/container/authz/capabilities.go", "policyDomain.Commander")
	assertFileLacks(t, root, "internal/apiserver/container/authz/capabilities.go", "policyDomain.Queryer")
	assertFileLacks(t, root, "internal/apiserver/container/authz/module.go", "CasbinAdapter *casbinInfra.CasbinAdapter")
	assertFileLacks(t, root, "internal/apiserver/container/module_graph.go", "AuthzModule.CasbinAdapter")
	assertFileLacks(t, root, "internal/apiserver/infra/casbin/adapter.go", "func (c *CasbinAdapter) Enforcer(")
	if matches, err := filepath.Glob(filepath.Join(root, "internal", "apiserver", "application", "authz", "version", "*.go")); err != nil {
		t.Fatal(err)
	} else if len(matches) > 0 {
		t.Fatalf("internal/apiserver/application/authz/version is retired; version changes must flow through PolicyChangeCommitter")
	}
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
		assertFourSegmentResourceValues(t, rel+" casbin_rule.v2", extractCasbinPolicyObjectsFromSQL(t, sql))
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
	assertAllowlistReasons(t, activeAuthzRootFacadeTestImports)
	retiredFacade := modulePath + "internal/apiserver/domain/authz"
	seen := map[string]struct{}{}
	scanImportsIncludingTests(t, filepath.Join(root, "internal", "apiserver"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		if !strings.HasSuffix(rel, "_test.go") {
			return
		}
		if rel == "internal/apiserver/domain/authz/model_test.go" || strings.Contains(rel, "compatibility") {
			return
		}
		for _, imp := range imports {
			if imp != retiredFacade {
				continue
			}
			if _, ok := activeAuthzRootFacadeTestImports[rel]; ok {
				seen[rel] = struct{}{}
				continue
			}
			t.Fatalf("%s imports root authz facade; new tests must use semantic child packages or be explicit compatibility tests", rel)
		}
	})
	assertAllowlistStillUsed(t, activeAuthzRootFacadeTestImports, seen)
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

func TestGRPCV2ContractsHaveRuntimeAndSDKCompileGuards(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	grpcRoot := filepath.Join(root, "api", "grpc", "iam")
	contracts := []struct {
		module           string
		proto            string
		goPackageAlias   string
		generatedPackage string
		serviceFile      string
		registerToken    string
		sdkFile          string
	}{
		{
			module:           "authn",
			proto:            "api/grpc/iam/authn/v2/authn.proto",
			goPackageAlias:   "authnv2",
			generatedPackage: "api/grpc/iam/authn/v2",
			serviceFile:      "internal/apiserver/transport/grpc/service/authn/service.go",
			registerToken:    "authnv2.RegisterAuthServiceServer",
			sdkFile:          "pkg/sdk/auth/client/client.go",
		},
		{
			module:           "authz",
			proto:            "api/grpc/iam/authz/v2/authz.proto",
			goPackageAlias:   "authzv2",
			generatedPackage: "api/grpc/iam/authz/v2",
			serviceFile:      "internal/apiserver/transport/grpc/service/authz/service.go",
			registerToken:    "authzv2.RegisterAuthorizationServiceServer",
			sdkFile:          "pkg/sdk/authz/client.go",
		},
		{
			module:           "identity",
			proto:            "api/grpc/iam/identity/v2/identity.proto",
			goPackageAlias:   "identityv2",
			generatedPackage: "api/grpc/iam/identity/v2",
			serviceFile:      "internal/apiserver/transport/grpc/service/identity/service.go",
			registerToken:    "identityv2.RegisterIdentityReadServer",
			sdkFile:          "pkg/sdk/identity/client.go",
		},
		{
			module:           "idp",
			proto:            "api/grpc/iam/idp/v2/idp.proto",
			goPackageAlias:   "idpv2",
			generatedPackage: "api/grpc/iam/idp/v2",
			serviceFile:      "internal/apiserver/transport/grpc/service/idp/service.go",
			registerToken:    "idpv2.RegisterIDPServiceServer",
			sdkFile:          "pkg/sdk/idp/client.go",
		},
	}

	for _, contract := range contracts {
		assertFileContains(t, root, contract.proto, "package iam."+contract.module+".v2;")
		assertFileContains(t, root, contract.proto, "github.com/FangcunMount/iam/v2/"+contract.generatedPackage+";"+contract.goPackageAlias)
		assertFileContains(t, root, filepath.ToSlash(filepath.Join(contract.generatedPackage, contract.module+".pb.go")), "package "+contract.goPackageAlias)
		assertFileContains(t, root, filepath.ToSlash(filepath.Join(contract.generatedPackage, contract.module+"_grpc.pb.go")), "package "+contract.goPackageAlias)
		assertFileContains(t, root, contract.serviceFile, "api/grpc/iam/"+contract.module+"/v2")
		assertFileContains(t, root, contract.serviceFile, contract.registerToken)
		assertFileContains(t, root, contract.sdkFile, "api/grpc/iam/"+contract.module+"/v2")
	}
	assertFileContains(t, root, "pkg/sdk/public_api_compile_test.go", `github.com/FangcunMount/iam/v2/pkg/sdk`)

	err := filepath.WalkDir(grpcRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".proto") {
			return nil
		}
		rel := filepath.ToSlash(mustRel(t, root, path))
		if strings.Contains(rel, "/v1/") {
			t.Fatalf("%s is a retired gRPC v1 contract; IAM public gRPC contracts must stay v2-only", rel)
		}
		if !strings.Contains(rel, "/v2/") {
			t.Fatalf("%s is not under a v2 contract directory", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
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

func TestAuthnTokenImplementationStaysOutOfDomain(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, rel := range []string{
		"internal/apiserver/domain/authn/token",
		"internal/apiserver/domain/authn/jwks",
		"internal/apiserver/infra/authentication",
		"internal/apiserver/infra/jwt",
	} {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(rel), "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) > 0 {
			t.Fatalf("%s is retired token implementation code; keep token use cases in application/authn/token and JWT encoding in infra/token/jwt", rel)
		}
	}

	forbiddenImports := map[string]struct{}{
		modulePath + "internal/apiserver/domain/authn/token":   {},
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
				t.Fatalf("%s imports %s; JWT libraries and retired token packages must stay behind infra/token/jwt or application token ports", rel, imp)
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
			if strings.HasPrefix(imp, modulePath+"internal/apiserver/domain/authn/") {
				t.Fatalf("%s imports %s; JWT infrastructure must implement application ports without depending on authn domain packages", rel, imp)
			}
		}
	})

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

func extractCasbinPolicyObjectsFromSQL(t *testing.T, sql string) []string {
	t.Helper()
	re := regexp.MustCompile(`(?is)SELECT\s+'p'\s*(?:AS\s+` + "`ptype`" + `)?\s*,\s*'[^']+'\s*(?:AS\s+` + "`v0`" + `)?\s*,\s*'[^']+'\s*(?:AS\s+` + "`v1`" + `)?\s*,\s*'([^']+)'\s*(?:AS\s+` + "`v2`" + `)?`)
	matches := re.FindAllStringSubmatch(sql, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, match[1])
	}
	if len(values) == 0 {
		t.Fatal("casbin p-rule bootstrap objects not found")
	}
	return values
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
