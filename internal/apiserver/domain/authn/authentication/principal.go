package authentication

import (
	"context"
	"fmt"

	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Principal 是认证成功后的运行时主体表达，是 Login 的领域终点。
// 职责：表示认证成功后的主体，包括：认证主体与身份入口、认证上下文、认证方式与认证域、认证方法引用、认证声明等
type Principal struct {
	// 认证主体与身份入口
	UserID          meta.ID
	LoginIdentityID meta.ID

	// 认证上下文
	TenantID  meta.ID
	SessionID string

	// 认证方式与认证域（可选）
	AuthMethod string         // 认证方式
	Realm      string         // 认证域
	AMR        []string       // 认证方法引用（可选）
	Claims     map[string]any // 认证声明（可选）
}

// AuthDecision 认证决策。
// 职责：表示认证结果，包括：认证是否成功、认证不通过的原因、认证成功后的主体、本次认证命中的登录身份、本次命中的凭据、凭据材料是否需要轮换等
type AuthDecision struct {
	OK bool

	// Code 表示认证不通过的业务原因。
	// 只有 OK=false 时有效。
	Code int

	// Principal 表示认证成功后的主体。
	// 只有 OK=true 时有效。
	Principal *Principal

	// LoginIdentityID 表示本次认证命中的登录身份。
	LoginIdentityID meta.ID

	// CredentialID 表示本次命中的凭据。
	// 可用于失败次数统计、锁定策略、成功登录审计。
	CredentialID meta.ID

	// 凭据材料是否需要轮换，例如密码 hash 参数升级。
	ShouldRotate bool
	NewMaterial  []byte
	NewAlgo      *string
}

// loginIdentityStatusFailureDecision 登录身份状态失败决策
// 参数：ctx 上下文, identityRepo 登录身份仓储, loginIdentityID 登录身份ID
// 返回：认证决策, 错误
// 职责：判断登录身份是否活动，如果不活动则返回认证失败决策
func loginIdentityStatusFailureDecision(ctx context.Context, identityRepo LoginIdentityRepository, loginIdentityID meta.ID) (*AuthDecision, error) {
	active, err := identityRepo.IsLoginIdentityActive(ctx, loginIdentityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get login identity status: %w", err)
	}
	if !active {
		return &AuthDecision{
			OK:              false,
			Code:            code.ErrLoginIdentityDisabled,
			LoginIdentityID: loginIdentityID,
		}, nil
	}
	return nil, nil
}
