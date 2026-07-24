package rest

import (
	"net/http"
	"time"

	readinessapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/readiness"
	"github.com/gin-gonic/gin"
)

// healthCheck is a compatibility summary. Deployment probes must use
// /healthz for liveness and /readyz for traffic readiness.
func (r *Router) healthCheck(c *gin.Context) {
	components := make(map[string]gin.H, len(r.deps.ModuleStatus.Modules))
	status := "healthy"
	for name, module := range r.deps.ModuleStatus.Modules {
		moduleStatus := "ok"
		if !module.Available {
			moduleStatus = "degraded"
			status = "degraded"
		}
		components[name] = gin.H{"status": moduleStatus}
	}
	c.JSON(http.StatusOK, gin.H{
		"status":     status,
		"components": components,
		"checked_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func (r *Router) readinessCheck(c *gin.Context) {
	if r.deps.Readiness == nil {
		c.JSON(http.StatusServiceUnavailable, readinessapp.Snapshot{
			Status:     "not_ready",
			Components: map[string]readinessapp.ComponentResult{},
			CheckedAt:  time.Now().UTC(),
		})
		return
	}
	snapshot, ready := r.deps.Readiness.Check(c.Request.Context())
	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, snapshot)
}
