package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDebugModulesCompatibilityViews(t *testing.T) {
	gin.SetMode(gin.TestMode)
	status := ModuleStatus{
		ContainerInitialized: true,
		Container: ModuleState{
			Bootstrapped: true,
			Available:    true,
		},
		Modules: map[string]ModuleState{
			moduleStateAuthn: {Bootstrapped: true, Available: true},
		},
		Authn: true,
	}
	router := &Router{deps: Deps{ModuleStatus: status}}

	tests := []struct {
		name       string
		query      string
		wantLegacy bool
		wantStates bool
	}{
		{name: "default combined", wantLegacy: true, wantStates: true},
		{name: "canonical", query: "?view=canonical", wantStates: true},
		{name: "legacy", query: "?view=legacy", wantLegacy: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/debug/modules"+tt.query, nil)

			router.debugModules(ctx)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", recorder.Code)
			}
			var body map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			_, hasLegacy := body["modules"]
			_, hasStates := body["module_states"]
			if hasLegacy != tt.wantLegacy || hasStates != tt.wantStates {
				t.Fatalf("response keys legacy=%t states=%t, want legacy=%t states=%t; body=%s", hasLegacy, hasStates, tt.wantLegacy, tt.wantStates, recorder.Body.String())
			}
		})
	}
}
