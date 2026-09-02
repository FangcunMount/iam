package authentication

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// Authenticator 认证器
type Authenticator struct {
	strategies map[CredentialKind]AuthStrategy
}

// NewAuthenticator 创建认证器
func NewAuthenticator(strategies ...AuthStrategy) *Authenticator {
	authenticator := &Authenticator{
		strategies: make(map[CredentialKind]AuthStrategy, len(strategies)),
	}
	for _, strategy := range strategies {
		authenticator.Register(strategy)
	}
	return authenticator
}

// Register 注册认证策略
func (a *Authenticator) Register(strategy AuthStrategy) {
	if a == nil || strategy == nil {
		return
	}
	if a.strategies == nil {
		a.strategies = make(map[CredentialKind]AuthStrategy)
	}
	a.strategies[strategy.Kind()] = strategy
}

// Authenticate 认证
// 统一流程：
// 1. 获取领域凭据类型对应的认证策略
// 2. 执行认证
func (a *Authenticator) Authenticate(ctx context.Context, proof AuthCredential) (AuthDecision, error) {
	l := logger.L(ctx)
	if proof == nil {
		return AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "authentication credential is required")
	}

	// 获取认证凭据类型
	credentialKind := proof.CredentialKind()
	if credentialKind == "" {
		return AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication credential kind: %s", credentialKind)
	}

	// 获取认证策略
	strategy := a.strategyFor(credentialKind)
	if strategy == nil {
		l.Errorw("不支持的认证场景", "action", logger.ActionLogin, "credential_kind", string(credentialKind))
		return AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication credential kind: %s", credentialKind)
	}

	// 执行认证
	decision, err := strategy.Authenticate(ctx, proof)
	if err != nil {
		l.Errorw("认证策略执行出错",
			"action", logger.ActionLogin,
			"credential_kind", string(credentialKind),
			"result", "failed",
			"error_category", "authentication",
			"retryable", false,
		)
		return AuthDecision{}, err
	}

	// 认证不通过
	if !decision.OK {
		l.Warnw("认证不通过（域层）", "action", logger.ActionLogin, "credential_kind", string(credentialKind), "code", decision.Code)
		return decision, nil
	}

	// 认证通过
	l.Debugw("认证成功（域层）", "action", logger.ActionLogin, "credential_kind", string(credentialKind), "user_id", decision.Principal.UserID.String(), "login_identity_id", decision.Principal.LoginIdentityID.String(), "tenant_id", decision.Principal.TenantID.String())

	return decision, nil
}

// strategyFor 获取认证策略
func (a *Authenticator) strategyFor(credentialKind CredentialKind) AuthStrategy {
	return a.strategies[credentialKind]
}
