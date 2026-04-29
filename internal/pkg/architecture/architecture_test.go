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

	"github.com/FangcunMount/iam/pkg/eventcatalog"
)

const modulePath = "github.com/FangcunMount/iam/"

var activeLegacyApplicationInfrastructureImports = map[string]string{}

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

func TestAssemblerModulesUseTypedDependencies(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "container", "assembler"), func(path, source string) {
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

	root := repoRoot(t)
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
	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "container", "assembler"), func(path, source string) {
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

	root := repoRoot(t)
	scanImports(t, filepath.Join(root, "internal", "apiserver", "container", "assembler"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, imp := range imports {
			if strings.HasPrefix(imp, modulePath+"internal/apiserver/transport/") {
				t.Fatalf("%s imports %s; assembler must expose application/domain capabilities and leave REST/gRPC construction to container transport deps builders", rel, imp)
			}
		}
	})
}

func TestAssemblerModulesDoNotExposeTransportFields(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	forbidden := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s*\w*Handler\s+\*`),
		regexp.MustCompile(`(?m)^\s*GRPCService\s+\*`),
	}
	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "container", "assembler"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, pattern := range forbidden {
			if match := pattern.FindString(source); match != "" {
				t.Fatalf("%s exposes transport field %q; expose application/domain capability methods instead", rel, strings.TrimSpace(match))
			}
		}
	})
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

func TestUCApplicationTransactionDomainCallsUseTxContext(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	forbiddenTokens := []string{
		"validator.ValidateCreate(ctx",
		"profileEditor.Rename(ctx",
		"profileEditor.Renickname(ctx",
		"profileEditor.UpdateContact(ctx",
		"profileEditor.UpdateIDCard(ctx",
		"profileService.Rename(ctx",
		"profileService.UpdateIDCard(ctx",
		"profileService.UpdateProfile(ctx",
		"profileService.UpdateHeightWeight(ctx",
		"managerService.Establish(ctx",
		"managerService.Revoke(ctx",
	}
	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "application", "uc"), func(path, source string) {
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
		"internal/apiserver/container/rest_deps.go":     {},
		"internal/apiserver/container/grpc_registry.go": {},
		"internal/apiserver/container/runtime_deps.go":  {},
		"internal/apiserver/container/module_graph.go":  {},
	}
	forbiddenTokens := []string{
		"AuthnModule.AuthHandler",
		"AuthnModule.AccountHandler",
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
		if strings.HasPrefix(rel, "internal/apiserver/container/assembler/") {
			return
		}
		for _, token := range forbiddenTokens {
			if strings.Contains(source, token) {
				t.Fatalf("%s directly navigates container module capability %q; add a container collector instead", rel, token)
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

	root := repoRoot(t)
	scanGoFiles(t, filepath.Join(root, "internal", "apiserver", "container", "assembler"), func(path string, file *ast.File) {
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

func TestGRPCV2ContractsAreNotAddedWithoutRuntime(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	grpcRoot := filepath.Join(root, "api", "grpc", "iam")
	err := filepath.WalkDir(grpcRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".proto") {
			return nil
		}
		rel := filepath.ToSlash(mustRel(t, root, path))
		if strings.Contains(rel, "/v2/") {
			t.Fatalf("%s is a gRPC v2 contract without a matching Phase 6 runtime registration and SDK compile-test migration", rel)
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
	outerCtxTxCall := regexp.MustCompile(`\b(?:tx(?:\.[A-Za-z0-9_]+)+|tx[A-Za-z0-9_]*\.[A-Za-z0-9_]+|editor\.[A-Za-z0-9_]+|statusManager\.[A-Za-z0-9_]+|lifecycler\.[A-Za-z0-9_]+)\(ctx\b`)
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

func TestUCLegacyChildGuardianshipModelDoesNotReturn(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	roots := []string{
		"api/grpc/iam/identity/v1",
		"api/rest/identity.v1.yaml",
		"configs/grpc_acl.yaml",
		"configs/mysql",
		"internal/apiserver/application/uc",
		"internal/apiserver/domain/uc",
		"internal/apiserver/infra/mysql",
		"internal/apiserver/transport/grpc/service/uc",
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

func TestUCSemanticServiceNamesDoNotRegress(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, relRoot := range []string{
		"internal/apiserver/application/uc",
		"internal/apiserver/domain/uc",
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
					t.Fatalf("%s contains retired UC service name %q; use semantic capabilities such as Creator, Editor, Directory, MyProfiles, Linker, or Commands", rel, token)
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
							t.Fatalf("%s declares %s; UC service names should express domain capability rather than Manager", rel, typeSpec.Name.Name)
						}
					}
				case *ast.FuncDecl:
					if strings.HasPrefix(d.Name.Name, "New") && strings.HasSuffix(d.Name.Name, "Manager") {
						t.Fatalf("%s declares %s; UC constructors should express domain capability rather than Manager", rel, d.Name.Name)
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
		t.Fatal("internal/apiserver/application/authn/register is retired; use application/authn/onboarding")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}

	assertFileContains(t, root, "api/grpc/iam/authn/v1/authn.proto", "service AccountOnboardingService")
	assertFileContains(t, root, "api/grpc/iam/authn/v1/authn.proto", "rpc CreateOperationAccount")
	assertFileLacks(t, root, "api/grpc/iam/authn/v1/authn.proto", "RegisterOperationAccount")
	assertFileLacks(t, root, "pkg/sdk/auth/client/client.go", "RegisterOperationAccount")
	assertFileContains(t, root, "pkg/sdk/auth/client/client.go", "CreateOperationAccount")

	assertFileContains(t, root, "api/grpc/iam/identity/v1/identity.proto", "rpc EstablishProfileLink")
	assertFileLacks(t, root, "api/grpc/iam/identity/v1/identity.proto", "CreateProfileLink")
	assertFileLacks(t, root, "pkg/sdk/identity/profile_link_command.go", "CreateProfileLink")

	for _, rel := range []string{
		"api/rest/authn.v1.yaml",
		"internal/apiserver/docs/swagger.yaml",
	} {
		assertFileContains(t, root, rel, "/authn/signups/wechat-miniprogram")
		assertFileLacks(t, root, rel, "/authn/accounts/wechat/register")
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
		modulePath + "internal/apiserver/domain/authn/token": {},
		modulePath + "internal/apiserver/infra/jwt":          {},
		"github.com/golang-jwt/jwt/v4":                       {},
		"github.com/golang-jwt/jwt/v5":                       {},
		"github.com/golang-jwt/jwt":                          {},
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

	forbiddenDomainTokens := []string{
		"AccessClaims",
		"AuthJWT" + "Token",
		"AMRJWT" + "Token",
		"JWT" + "TokenCredential",
		"JWT" + "TokenAuthStrategy",
		"FlattenClaimsFor" + "JWT",
	}
	scanGoSources(t, filepath.Join(root, "internal", "apiserver", "domain", "authn"), func(path, source string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		if strings.HasPrefix(rel, "internal/apiserver/domain/authn/jwks/") {
			return
		}
		for _, token := range forbiddenDomainTokens {
			if strings.Contains(source, token) {
				t.Fatalf("%s contains retired JWT-shaped domain token %q; keep JWT claims and strategies in infra/token/jwt", rel, token)
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
			t.Fatalf("%s contains retired UC model token %q", rel, token)
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
		"internal/apiserver/application/uc/child",
		"internal/apiserver/application/uc/guardianship",
		"internal/apiserver/domain/uc/child",
		"internal/apiserver/domain/uc/guardianship",
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
			t.Fatalf("%s contains retired UC model token %q", rel, token)
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
