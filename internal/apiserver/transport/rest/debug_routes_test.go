package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDebugModulesReturnsCanonicalStateOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	status := ModuleStatus{
		Container: ModuleState{Bootstrapped: true, Available: true},
		Modules: map[string]ModuleState{
			moduleStateAuthn: {Bootstrapped: true, Available: true},
		},
	}
	router := &Router{deps: Deps{ModuleStatus: status}}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/debug/modules?view=legacy", nil)

	router.debugModules(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["module_states"]; !ok {
		t.Fatalf("canonical module_states missing: %s", recorder.Body.String())
	}
	for _, legacy := range []string{"container_initialized", "modules"} {
		if _, ok := body[legacy]; ok {
			t.Fatalf("legacy key %q remains: %s", legacy, recorder.Body.String())
		}
	}
}
