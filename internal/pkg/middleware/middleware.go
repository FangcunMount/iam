package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	gindump "github.com/tpkeeper/gin-dump"
)

// Middlewares store registered middlewares.
// 存储注册的中间件
var Middlewares = defaultMiddlewares()

// NoCache 是一个中间件函数，用于添加头信息，防止客户端缓存 HTTP 响应
func NoCache(c *gin.Context) {
	c.Header("Cache-Control", "no-cache, no-store, max-age=0, must-revalidate, value")
	c.Header("Expires", "Thu, 01 Jan 1970 00:00:00 GMT")
	c.Header("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	c.Next()
}

// Options 是一个中间件函数，用于添加头信息，处理 OPTIONS 请求，并中止中间件链和结束请求
// 注意：生产环境若前有 Nginx/CORS，勿在此处再写 Access-Control-Allow-Origin，否则会与网关叠成「多值」导致浏览器报错。
func Options(c *gin.Context) {
	if c.Request.Method != "OPTIONS" {
		c.Next()
	} else {
		// 仅本地直连 apiserver 时用；生产由 Nginx 处理 OPTIONS 时不会进到这里
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "authorization, origin, content-type, accept, tenant_id, tenant-id")
		c.Header("Allow", "HEAD,GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Content-Type", "application/json")
		c.AbortWithStatus(http.StatusOK)
	}
}

// Secure 是一个中间件函数，用于添加安全头信息和资源访问头信息
// 不在此设置 CORS：由前置 Nginx（如 iam.fangcunmount.cn.conf 的 map + add_header）统一出 Allow-Origin，避免与 Gin 重复。
func Secure(c *gin.Context) {
	c.Header("X-Frame-Options", "DENY")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-XSS-Protection", "1; mode=block")

	if c.Request.TLS != nil {
		c.Header("Strict-Transport-Security", "max-age=31536000")
	}
}

// defaultMiddlewares 返回默认的中间件
func defaultMiddlewares() map[string]gin.HandlerFunc {
	return map[string]gin.HandlerFunc{
		"recovery": gin.Recovery(),
		"secure":   Secure,
		"options":  Options,
		"nocache":  NoCache,
		"tracing":  Tracing(), // 链路追踪中间件（整合了 request_id 生成）
		"dump":     gindump.Dump(),
	}
}
