package requestctx

import (
	"github.com/gin-gonic/gin"
)

// SetTokenID 设置Token ID到请求上下文中
func SetTokenID(c *gin.Context, tokenID string) {
	if c == nil || tokenID == "" {
		return
	}
	c.Set(KeyTokenID, tokenID)
}

// TokenIDString 从请求上下文中获取Token ID字符串
func TokenIDString(c *gin.Context) (string, bool) {
	return stringValue(c, KeyTokenID)
}

func RequestIDString(c *gin.Context) (string, bool) {
	return stringValue(c, KeyRequestID)
}
