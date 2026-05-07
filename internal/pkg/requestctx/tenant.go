package requestctx

import (
	"github.com/gin-gonic/gin"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/pkg/tenant"
)

// SetTenantID 设置租户ID到请求上下文中
func SetTenantID(c *gin.Context, tenantID string) {
	if c == nil || tenantID == "" {
		return
	}
	c.Set(KeyTenantID, tenantID)
}

// TenantIDOrDefault 从请求上下文中获取租户ID，如果不存在则返回默认租户ID
func TenantIDOrDefault(c *gin.Context) string {
	id, ok := TenantIDString(c)
	if !ok {
		return tenant.DefaultID
	}
	return id
}

// TenantIDString 从请求上下文中获取租户ID字符串，如果不存在或不是字符串则返回空字符串和false
func TenantIDString(c *gin.Context) (string, bool) {
	return stringValue(c, KeyTenantID)
}

// RequiredTenantID 从请求上下文中获取租户ID，如果不存在或无效则返回错误
func RequiredTenantID(c *gin.Context) (string, error) {
	if c == nil {
		return "", perrors.WithCode(code.ErrTokenInvalid, "request context is nil")
	}
	id, ok := TenantIDString(c)
	if !ok {
		return "", perrors.WithCode(code.ErrTokenInvalid, "tenant id not found in context")
	}
	return id, nil
}
