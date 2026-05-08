package authentication

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// AuthStrategy 认证策略（领域服务接口）
type AuthStrategy interface {
	Kind() credDomain.CredentialType
	Authenticate(ctx context.Context, proof AuthCredential) (AuthDecision, error)
}

// Authenticator 认证器
type Authenticator struct {
	strategies map[credDomain.CredentialType]AuthStrategy
}

// NewAuthenticator 创建认证器
func NewAuthenticator(strategies ...AuthStrategy) *Authenticator {
	authenticator := &Authenticator{
		strategies: make(map[credDomain.CredentialType]AuthStrategy, len(strategies)),
	}
	for _, strategy := range strategies {
		authenticator.Register(strategy)
	}
	return authenticator
}

func (a *Authenticator) Register(strategy AuthStrategy) {
	if a == nil || strategy == nil {
		return
	}
	if a.strategies == nil {
		a.strategies = make(map[credDomain.CredentialType]AuthStrategy)
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
	credentialType := proof.CredentialType()

	l.Debugw("开始认证流程（域层）",
		"action", logger.ActionLogin,
		"credential_type", string(credentialType),
		"amr", []string{string(credentialType)},
		"claims", make(map[string]any),
	)

	l.Debugw("认证凭据构建完成",
		"action", logger.ActionLogin,
		"credential_type", credentialType,
		"amr", []string{string(credentialType)},
		"claims", make(map[string]any),
	)

	strategy := a.strategyFor(credentialType)
	if strategy == nil {
		l.Errorw("不支持的认证场景",
			"action", logger.ActionLogin,
			"credential_type", string(credentialType),
		)
		return AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication credential type: %s", credentialType)
	}

	l.Debugw("认证策略已创建",
		"action", logger.ActionLogin,
		"credential_type", string(credentialType),
		"strategy", credentialType,
	)

	// 执行认证
	l.Debugw("开始执行认证策略",
		"action", logger.ActionLogin,
		"credential_type", string(credentialType),
	)

	decision, err := strategy.Authenticate(ctx, proof)
	if err != nil {
		l.Errorw("认证策略执行出错",
			"action", logger.ActionLogin,
			"credential_type", string(credentialType),
			"error", err.Error(),
		)
		return AuthDecision{}, err
	}

	if !decision.OK {
		l.Warnw("认证不通过（域层）",
			"action", logger.ActionLogin,
			"credential_type", string(credentialType),
			"code", decision.Code,
		)
		return decision, nil
	}

	l.Debugw("认证成功（域层）",
		"action", logger.ActionLogin,
		"credential_type", string(credentialType),
		"user_id", decision.Principal.UserID.String(),
		"account_id", decision.Principal.AccountID.String(),
		"tenant_id", decision.Principal.TenantID.String(),
	)

	return decision, nil
}

func (a *Authenticator) strategyFor(credentialType credDomain.CredentialType) AuthStrategy {
	if a == nil {
		return nil
	}
	return a.strategies[credentialType]
}
