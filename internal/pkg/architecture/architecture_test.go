package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const modulePath = "github.com/FangcunMount/iam/"

var allowedDomainInfrastructureImports = map[string]string{}

var allowedApplicationInfrastructureImports = map[string]string{
	"internal/apiserver/application/cachegovernance/jwks_inspector.go:github.com/FangcunMount/iam/internal/apiserver/infra/cache": "legacy_cache_inspector_model_to_move_to_port",
	"internal/apiserver/application/cachegovernance/model.go:github.com/FangcunMount/iam/internal/apiserver/infra/cache":          "legacy_cache_inspector_model_to_move_to_port",
	"internal/apiserver/application/cachegovernance/service.go:github.com/FangcunMount/iam/internal/apiserver/infra/cache":        "legacy_cache_inspector_model_to_move_to_port",
	"internal/apiserver/application/suggest/service.go:github.com/FangcunMount/iam/internal/apiserver/infra/suggest/search":       "legacy_direct_search_adapter_to_hide_behind_port",
	"internal/apiserver/application/suggest/updater.go:github.com/FangcunMount/iam/internal/apiserver/infra/suggest/search":       "legacy_direct_search_adapter_to_hide_behind_port",
	"internal/apiserver/application/uc/testutil/db.go:github.com/FangcunMount/iam/internal/apiserver/infra/mysql/child":           "application_test_support",
	"internal/apiserver/application/uc/testutil/db.go:github.com/FangcunMount/iam/internal/apiserver/infra/mysql/guardianship":    "application_test_support",
	"internal/apiserver/application/uc/testutil/db.go:github.com/FangcunMount/iam/internal/apiserver/infra/mysql/user":            "application_test_support",
}

func TestDomainPackagesDoNotAddInfrastructureDependencies(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	assertAllowlistReasons(t, allowedDomainInfrastructureImports)
	seen := map[string]struct{}{}

	scanImports(t, filepath.Join(root, "internal", "apiserver", "domain"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, imp := range imports {
			if !isDomainForbiddenImport(imp) {
				continue
			}
			key := rel + ":" + imp
			if _, ok := allowedDomainInfrastructureImports[key]; ok {
				seen[key] = struct{}{}
				continue
			}
			t.Fatalf("%s imports %s; domain must stay independent from infrastructure and database packages", rel, imp)
		}
	})
	assertAllowlistStillUsed(t, allowedDomainInfrastructureImports, seen)
}

func TestApplicationPackagesDoNotAddTransportOrInfrastructureDependencies(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	assertAllowlistReasons(t, allowedApplicationInfrastructureImports)
	seen := map[string]struct{}{}

	scanImports(t, filepath.Join(root, "internal", "apiserver", "application"), func(path string, imports []string) {
		rel := filepath.ToSlash(mustRel(t, root, path))
		for _, imp := range imports {
			if strings.HasPrefix(imp, modulePath+"internal/apiserver/interface/") {
				t.Fatalf("%s imports %s; application layer must not depend on transport/interface implementations", rel, imp)
			}
			if !isApplicationForbiddenImport(imp) {
				continue
			}
			key := rel + ":" + imp
			if _, ok := allowedApplicationInfrastructureImports[key]; ok {
				seen[key] = struct{}{}
				continue
			}
			t.Fatalf("%s imports %s; add a port/factory seam or document a temporary allowlist reason", rel, imp)
		}
	})
	assertAllowlistStillUsed(t, allowedApplicationInfrastructureImports, seen)
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
				if strings.HasPrefix(imp, modulePath+"internal/apiserver/interface/") {
					t.Fatalf("%s imports %s; data access packages must not depend on transport/interface implementations", rel, imp)
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

func isDomainForbiddenImport(imp string) bool {
	return imp == "gorm.io/gorm" ||
		strings.HasPrefix(imp, modulePath+"internal/apiserver/infra/") ||
		strings.HasPrefix(imp, modulePath+"internal/pkg/database") ||
		strings.HasPrefix(imp, modulePath+"internal/pkg/migration") ||
		strings.HasPrefix(imp, modulePath+"internal/apiserver/interface/")
}

func isApplicationForbiddenImport(imp string) bool {
	return strings.HasPrefix(imp, modulePath+"internal/apiserver/infra/") ||
		strings.HasPrefix(imp, modulePath+"internal/pkg/database") ||
		strings.HasPrefix(imp, modulePath+"internal/pkg/migration")
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
