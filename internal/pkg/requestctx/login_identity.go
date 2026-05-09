package requestctx

import (
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/gin-gonic/gin"
)

// SetLoginIdentityID 设置登录身份ID到请求上下文中
func SetLoginIdentityID(c *gin.Context, id meta.ID) {
	if c == nil || id.IsZero() {
		return
	}
	c.Set(KeyLoginIdentityID, id)
}

// LoginIdentityID 从请求上下文中获取登录身份ID
func LoginIdentityID(c *gin.Context) (meta.ID, bool) {
	return idValue(c, KeyLoginIdentityID)
}
