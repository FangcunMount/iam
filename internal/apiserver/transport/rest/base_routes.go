package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	openapiFS "github.com/FangcunMount/iam/v2/api"
	swaggerui "github.com/FangcunMount/iam/v2/web/swagger-ui"
)

func (r *Router) registerBaseRoutes(engine *gin.Engine) {
	engine.GET("/health", r.healthCheck)
	engine.GET("/ping", r.ping)
	engine.GET("/debug/routes", r.debugRoutes)
	engine.GET("/debug/modules", r.debugModules)

	engine.StaticFS("/openapi", http.FS(openapiFS.RestFS))
	engine.StaticFS("/swagger", http.FS(swaggerui.DistFS))

	publicAPI := engine.Group("/api/v2/public")
	{
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

func (r *Router) ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
		"status":  "ok",
		"router":  "centralized",
		"auth":    "enabled",
	})
}
