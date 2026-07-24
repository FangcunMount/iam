package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestHealthzIsProcessOnlyAndNeedsNoExternalDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config := NewConfig()
	config.EnableMetrics = false
	config.EnableProfiling = false
	server, err := config.Complete().New()
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"status":"ok"`)
}
