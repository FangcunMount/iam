package requestctx

import (
	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

const (
	KeyClaims          = "claims"
	KeyUserID          = "user_id"
	KeyLoginIdentityID = "login_identity_id"
	KeyTenantID        = "tenant_id"
	KeyOrgID           = "org_id"
	KeyTokenID         = "token_id"
	KeyRequestID       = "request_id"
)

func stringValue(c *gin.Context, key string) (string, bool) {
	if c == nil {
		return "", false
	}
	raw, exists := c.Get(key)
	if !exists {
		return "", false
	}
	value, ok := raw.(string)
	if !ok || value == "" {
		return "", false
	}
	return value, true
}

func idValue(c *gin.Context, key string) (meta.ID, bool) {
	if c == nil {
		return 0, false
	}
	raw, exists := c.Get(key)
	if !exists {
		return 0, false
	}
	switch value := raw.(type) {
	case meta.ID:
		if value.IsZero() {
			return 0, false
		}
		return value, true
	case string:
		id, err := meta.ParseID(value)
		if err != nil || id.IsZero() {
			return 0, false
		}
		return id, true
	default:
		return 0, false
	}
}
