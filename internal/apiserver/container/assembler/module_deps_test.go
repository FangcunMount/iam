package assembler

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
	"gorm.io/gorm"
)

func TestSuggestModuleInitializeWithDepsDisabledDoesNotRequireDB(t *testing.T) {
	module := NewSuggestModule()

	if err := module.InitializeWithDeps(SuggestModuleDeps{Config: appsuggest.Config{Enable: false}}); err != nil {
		t.Fatalf("InitializeWithDeps() error = %v", err)
	}
	if module.IsInitialized() {
		t.Fatalf("IsInitialized() = true, want false when suggest is disabled")
	}
}

func TestSuggestModuleInitializeWithDepsEnabledRequiresDB(t *testing.T) {
	module := NewSuggestModule()

	if err := module.InitializeWithDeps(SuggestModuleDeps{Config: appsuggest.Config{Enable: true}}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing DB error")
	}
}

func TestUserModuleInitializeWithDepsRequiresDB(t *testing.T) {
	module := NewUserModule()

	if err := module.InitializeWithDeps(UserModuleDeps{}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing DB error")
	}
}

func TestIDPModuleInitializeWithDepsRequiresDependencies(t *testing.T) {
	module := NewIDPModule()

	if err := module.InitializeWithDeps(IDPModuleDeps{}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing DB error")
	}
}

func TestAuthzModuleInitializeWithDepsRequiresDB(t *testing.T) {
	module := NewAuthzModule()

	if err := module.InitializeWithDeps(AuthzModuleDeps{}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing DB error")
	}
}

func TestAuthzModuleInitializeWithDepsRequiresEventStager(t *testing.T) {
	module := NewAuthzModule()

	if err := module.InitializeWithDeps(AuthzModuleDeps{DB: &gorm.DB{}}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing event stager error")
	}
}

func TestAuthzDefaultModelPathUsesFourSegmentMatcher(t *testing.T) {
	modelPath := authzModelPath(" ")
	if modelPath != "configs/casbin_model.conf" {
		t.Fatalf("authzModelPath(empty) = %q, want configs/casbin_model.conf", modelPath)
	}

	data, err := os.ReadFile(filepath.Join(assemblerRepoRoot(t), filepath.FromSlash(modelPath)))
	if err != nil {
		t.Fatal(err)
	}
	model := string(data)
	for _, token := range []string{"resourceMatch(r.obj, p.obj)", "actionMatch(r.act, p.act)", "scopeMatch(r.scope, p.scope)"} {
		if !strings.Contains(model, token) {
			t.Fatalf("default authz model %s does not contain %q", modelPath, token)
		}
	}
	for _, token := range []string{"keyMatch(r.obj, p.obj)", "regexMatch(r.act, p.act)"} {
		if strings.Contains(model, token) {
			t.Fatalf("default authz model %s still contains legacy matcher %q", modelPath, token)
		}
	}
}

func assemblerRepoRoot(t *testing.T) string {
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
