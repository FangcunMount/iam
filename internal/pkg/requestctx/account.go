package requestctx

import (
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/gin-gonic/gin"
)

// SetAccountID 设置账户ID到请求上下文中
func SetAccountID(c *gin.Context, id meta.ID) {
	if c == nil || id.IsZero() {
		return
	}
	c.Set(KeyAccountID, id)
}

// AccountID 从请求上下文中获取账户ID
func AccountID(c *gin.Context) (meta.ID, bool) {
	return idValue(c, KeyAccountID)
}
