package rest

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (r *Router) healthCheck(c *gin.Context) {
	authEnabled := r.deps.ModuleStatus.AuthEnabled
	authzRuntimeHealthy := true
	authzRuntimeError := ""
	var authzReloadedAt time.Time

	if r.deps.Authz.HealthReporter != nil {
		var err error
		authzRuntimeHealthy, err, authzReloadedAt = r.deps.Authz.HealthReporter.ReloadHealth()
		if err != nil {
			authzRuntimeError = err.Error()
		}
	}

	status := "healthy"
	statusCode := http.StatusOK
	if !authEnabled || !authzRuntimeHealthy {
		status = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	response := gin.H{
		"status":       status,
		"version":      "1.0.0",
		"discovery":    "auto",
		"architecture": "hexagonal",
		"router":       "centralized",
		"auth":         map[bool]string{true: "enabled", false: "disabled"}[authEnabled],
		"components": gin.H{
			"domain":      "user",
			"ports":       "storage",
			"adapters":    "mysql, http",
			"application": "user_service",
		},
		"auth_system": gin.H{
			"type":    "jwt",
			"enabled": authEnabled,
			"module":  "authn (DDD 4-layer)",
		},
		"authz_runtime": gin.H{
			"healthy":    authzRuntimeHealthy,
			"last_error": authzRuntimeError,
			"reloaded_at": func() string {
				if authzReloadedAt.IsZero() {
					return ""
				}
				return authzReloadedAt.Format(time.RFC3339)
			}(),
		},
	}

	c.JSON(statusCode, response)
}
