package authentication

import "github.com/FangcunMount/iam/v3/internal/pkg/meta"

// Principal 是认证成功后的运行时主体表达，是 Login 的领域终点。
type Principal struct {
	UserID          meta.ID
	LoginIdentityID meta.ID

	TenantID  meta.ID
	SessionID string

	// AuthContext 是认证上下文的权威表达。
	AuthContext AuthenticationContext
	// TokenContext 是允许进入 Session/JWT 的最小强类型投影。
	TokenContext TokenContext
}

// TokenContext 表达认证结果中允许进入在线会话和访问令牌的业务上下文。
type TokenContext struct {
	TenantDomain string
	OrgID        meta.ID
	Attributes   map[string]string
}

func (c TokenContext) Clone() TokenContext {
	out := c
	if len(c.Attributes) > 0 {
		out.Attributes = make(map[string]string, len(c.Attributes))
		for key, value := range c.Attributes {
			out.Attributes[key] = value
		}
	}
	return out
}

// ApplyAuthContext 将认证上下文写入 Principal，并同步兼容字段。
func (p *Principal) ApplyAuthContext(ctx AuthenticationContext) {
	if p == nil {
		return
	}
	p.AuthContext = ctx.Clone()
}
