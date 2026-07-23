package authn

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequireSeedMockSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		secret     string
		wantStatus int
	}{
		{name: "missing", wantStatus: http.StatusUnauthorized},
		{name: "wrong", secret: "wrong-secret", wantStatus: http.StatusUnauthorized},
		{name: "correct", secret: "expected-secret", wantStatus: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := gin.New()
			engine.POST(
				"/internal",
				RequireSeedMockSecret("expected-secret"),
				func(c *gin.Context) { c.Status(http.StatusNoContent) },
			)

			req := httptest.NewRequest(http.MethodPost, "/internal", nil)
			if tt.secret != "" {
				req.Header.Set(SeedMockSecretHeader, tt.secret)
			}
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
		})
	}
}
