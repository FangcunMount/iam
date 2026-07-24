package middleware

import (
	"net/http"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/gin-gonic/gin"
)

var apiLogSkipPaths = map[string]struct{}{
	"/health":      {},
	"/healthz":     {},
	"/readyz":      {},
	"/metrics":     {},
	"/favicon.ico": {},
}

// APILogger records HTTP transport metadata only. Request headers, query
// values, bodies, response headers, bodies and handler error messages are
// intentionally never inspected.
func APILogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, skip := apiLogSkipPaths[c.Request.URL.Path]; skip {
			c.Next()
			return
		}

		started := time.Now()
		requestID := c.GetString(XRequestIDKey)
		reqLogger := logger.NewRequestLogger(c.Request.Context(),
			log.String(logger.FieldMethod, c.Request.Method),
			log.String(logger.FieldPath, c.Request.URL.Path),
			log.String(logger.FieldClientIP, c.ClientIP()),
			log.String(logger.FieldRequestID, requestID),
		)
		ctx := logger.WithLogger(c.Request.Context(), reqLogger)
		c.Request = c.Request.WithContext(ctx)

		log.HTTP("HTTP Request Started",
			append([]log.Field{
				log.String("event", "request_start"),
				log.String("request_id", requestID),
				log.String("method", c.Request.Method),
				log.String("path", c.Request.URL.Path),
				log.String("client_ip", c.ClientIP()),
			}, log.TraceFields(ctx)...)...,
		)

		c.Next()

		fields := []log.Field{
			log.String("event", "request_end"),
			log.String("request_id", requestID),
			log.String("method", c.Request.Method),
			log.String("path", c.FullPath()),
			log.Int("status_code", c.Writer.Status()),
			log.Int("response_size", c.Writer.Size()),
			log.Int64("duration_ms", time.Since(started).Milliseconds()),
		}
		fields = append(fields, log.TraceFields(ctx)...)
		switch {
		case c.Writer.Status() >= http.StatusInternalServerError:
			log.HTTPError("HTTP Request Completed with Server Error", fields...)
		case c.Writer.Status() >= http.StatusBadRequest:
			log.HTTPWarn("HTTP Request Completed with Client Error", fields...)
		default:
			log.HTTPDebug("HTTP Request Completed Successfully", fields...)
		}
	}
}
