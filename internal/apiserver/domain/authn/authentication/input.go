package authentication

import "github.com/FangcunMount/iam/v2/internal/pkg/meta"

// AuthDecision 认证决策。
type AuthDecision struct {
	OK bool

	// Code 表示认证不通过的业务原因。
	// 只有 OK=false 时有效。
	Code int

	// Principal 表示认证成功后的主体。
	// 只有 OK=true 时有效。
	Principal *Principal

	// CredentialID 表示本次命中的凭据。
	// 可用于失败次数统计、锁定策略、成功登录审计。
	CredentialID meta.ID

	// 凭据材料是否需要轮换，例如密码 hash 参数升级。
	ShouldRotate bool
	NewMaterial  []byte
	NewAlgo      *string
}
