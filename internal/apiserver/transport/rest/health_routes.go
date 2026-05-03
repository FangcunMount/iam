package rest

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// healthCheck 健康检查路由
func (r *Router) healthCheck(c *gin.Context) {
	// 获取认证状态
	authEnabled := r.deps.ModuleStatus.AuthEnabled
	// 获取认证运行时健康状态
	authzRuntimeHealthy := true
	// 获取认证运行时错误
	authzRuntimeError := ""
	// 获取认证运行时重载时间
	var authzReloadedAt time.Time

	// 如果认证运行时健康报告器不为空，则获取认证运行时健康状态
	if r.deps.Authz.HealthReporter != nil {
		// 获取认证运行时健康状态
		var err error
		authzRuntimeHealthy, err, authzReloadedAt = r.deps.Authz.HealthReporter.ReloadHealth()
		// 如果认证运行时健康状态错误，则设置认证运行时错误
		if err != nil {
			authzRuntimeError = err.Error()
		}
	}

	// 设置状态
	status := "healthy"
	// 设置状态码
	statusCode := http.StatusOK
	if !authEnabled || !authzRuntimeHealthy {
		// 如果认证状态或认证运行时健康状态不健康，则设置状态为降级
		status = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	// 创建响应
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
