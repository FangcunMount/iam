package requestctx

import (
	"strconv"

	"github.com/gin-gonic/gin"

	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
)

// SetOrgID 将 JWT 透传的业务 org_id 写入请求上下文（非 IAM 领域字段）。
func SetOrgID(c *gin.Context, orgID uint64) {
	if c == nil || orgID == 0 {
		return
	}
	c.Set(KeyOrgID, strconv.FormatUint(orgID, 10))
}

// BusinessOrgID 读取 token 透传的业务组织 ID。
// IAM 不定义默认 org；无 claim 时返回 ok=false。
func BusinessOrgID(c *gin.Context) (uint64, bool) {
	if c == nil {
		return 0, false
	}
	if claims, ok := tokenClaims(c); ok && !claims.OrgID.IsZero() {
		return claims.OrgID.Uint64(), true
	}
	if raw, ok := stringValue(c, KeyOrgID); ok {
		if n, err := strconv.ParseUint(raw, 10, 64); err == nil {
			return n, true
		}
	}
	return 0, false
}

func tokenClaims(c *gin.Context) (*tokenapp.TokenClaims, bool) {
	raw, ok := Claims(c)
	if !ok || raw == nil {
		return nil, false
	}
	claims, ok := raw.(*tokenapp.TokenClaims)
	return claims, ok
}
