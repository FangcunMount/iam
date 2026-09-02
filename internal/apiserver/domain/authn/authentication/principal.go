package authentication

import "github.com/FangcunMount/iam/v3/internal/pkg/meta"

// Principal 是认证成功后的运行时主体表达，是 Login 的领域终点。
type Principal struct {
	UserID          meta.ID
	LoginIdentityID meta.ID

	TenantID  meta.ID
	SessionID string

	AuthMethod string
	Realm      string
	AMR        []string
	Claims     map[string]any
}
