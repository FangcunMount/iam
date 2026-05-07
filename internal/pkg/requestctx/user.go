package requestctx

import (
	"github.com/gin-gonic/gin"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// SetUserID 设置用户ID到请求上下文中
func SetUserID(c *gin.Context, id meta.ID) {
	if c == nil || id.IsZero() {
		return
	}
	c.Set(KeyUserID, id)
}

// UserID 从请求上下文中获取用户ID
func UserID(c *gin.Context) (meta.ID, bool) {
	return idValue(c, KeyUserID)
}

// RequiredUserID 从请求上下文中获取用户ID，如果不存在或无效则返回错误
func RequiredUserID(c *gin.Context) (meta.ID, error) {
	id, ok := UserID(c)
	if !ok {
		return 0, perrors.WithCode(code.ErrTokenInvalid, "user id not found in context")
	}
	return id, nil
}
