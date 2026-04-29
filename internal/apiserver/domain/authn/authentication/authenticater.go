package authentication

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// AuthStrategy 认证策略（领域服务接口）
type AuthStrategy interface {
	Kind() Scenario
	Authenticate(ctx context.Context, credential AuthCredential) (AuthDecision, error)
}

// Authenticator 认证器
type Authenticator struct {
	strategies map[Scenario]AuthStrategy
}

// NewAuthenticator 创建认证器
func NewAuthenticator(strategies ...AuthStrategy) *Authenticator {
	authenticator := &Authenticator{
		strategies: make(map[Scenario]AuthStrategy, len(strategies)),
	}
	for _, strategy := range strategies {
		if strategy == nil {
			continue
		}
		authenticator.strategies[strategy.Kind()] = strategy
	}
	return authenticator
}

// Authenticate 认证
// 统一流程：
// 1. 获取领域凭据所属的认证策略
// 2. 执行认证
func (a *Authenticator) Authenticate(ctx context.Context, credential AuthCredential) (AuthDecision, error) {
	l := logger.L(ctx)
	if credential == nil {
		return AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "authentication credential is required")
	}
	scenario := credential.Scenario()

	l.Debugw("开始认证流程（域层）",
		"action", logger.ActionLogin,
		"scenario", string(scenario),
		"amr", []string{string(scenario)},
		"claims", make(map[string]any),
	)

	l.Debugw("认证凭据构建完成",
		"action", logger.ActionLogin,
		"scenario", string(scenario),
		"credential_type", scenario,
		"amr", []string{string(scenario)},
		"claims", make(map[string]any),
	)

	strategy := a.strategyFor(scenario)
	if strategy == nil {
		l.Errorw("不支持的认证场景",
			"action", logger.ActionLogin,
			"scenario", string(scenario),
		)
		return AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication scenario: %s", scenario)
	}

	l.Debugw("认证策略已创建",
		"action", logger.ActionLogin,
		"scenario", string(scenario),
		"strategy", scenario,
	)

	// 执行认证
	l.Debugw("开始执行认证策略",
		"action", logger.ActionLogin,
		"scenario", string(scenario),
	)

	decision, err := strategy.Authenticate(ctx, credential)
	if err != nil {
		l.Errorw("认证策略执行出错",
			"action", logger.ActionLogin,
			"scenario", string(scenario),
			"error", err.Error(),
		)
		return AuthDecision{}, err
	}

	if !decision.OK {
		l.Warnw("认证不通过（域层）",
			"action", logger.ActionLogin,
			"scenario", string(scenario),
			"err_code", string(decision.ErrCode),
		)
		return decision, nil
	}

	l.Debugw("认证成功（域层）",
		"action", logger.ActionLogin,
		"scenario", string(scenario),
		"user_id", decision.Principal.UserID.String(),
		"account_id", decision.Principal.AccountID.String(),
		"tenant_id", decision.Principal.TenantID.String(),
	)

	return decision, nil
}

func (a *Authenticator) strategyFor(scenario Scenario) AuthStrategy {
	if a == nil {
		return nil
	}
	return a.strategies[scenario]
}
