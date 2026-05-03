package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	openapiFS "github.com/FangcunMount/iam/v2/api"
	swaggerui "github.com/FangcunMount/iam/v2/web/swagger-ui"
)

// registerBaseRoutes 注册基础路由
func (r *Router) registerBaseRoutes(engine *gin.Engine) {
	// 注册健康检查路由
	engine.GET("/health", r.healthCheck)
	// 注册连通性检查路由
	engine.GET("/ping", r.ping)
	// 注册路由调试路由
	engine.GET("/debug/routes", r.debugRoutes)
	// 注册模块调试路由
	engine.GET("/debug/modules", r.debugModules)

	// 注册OpenAPI静态文件
	engine.StaticFS("/openapi", http.FS(openapiFS.RestFS))
	// 注册Swagger静态文件
	engine.StaticFS("/swagger", http.FS(swaggerui.DistFS))

	// 创建公共API组
	publicAPI := engine.Group("/api/v2/public")
	{
		// 注册公共API信息路由
		publicAPI.GET("/info", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"service":     "iam-apiserver",
				"version":     "1.0.0",
				"description": "IAM API Server",
				"swagger":     "/swagger/index.html",
			})
		})
	}
}

// ping 连通性检查路由
func (r *Router) ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
		"status":  "ok",
		"router":  "centralized",
		"auth":    "enabled",
	})
}
