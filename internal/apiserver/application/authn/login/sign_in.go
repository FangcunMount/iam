package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	tokenapp "github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/FangcunMount/iam/pkg/tenant"
)

// SignIn 编排一次登录：选择登录方式、准备领域 proof、调用领域认证并签发令牌。
type SignIn struct {
	tokenIssuer         tokenapp.Issuer
	methodSelector      MethodSelector
	domainAuthenticator *authentication.Authenticator
	failureTranslator   AuthFailureTranslator
}

func (s *SignIn) Execute(ctx context.Context, cmd SignInCommand) (*SignInResult, error) {
	l := logger.L(ctx)

	attempt, err := s.selectMethod(ctx, cmd)
	if err != nil {
		l.Warnw("认证准备失败",
			"action", logger.ActionLogin,
			"error", err.Error(),
		)
		return nil, err
	}

	l.Debugw("开始认证流程",
		"action", logger.ActionLogin,
		"scenario", string(attempt.Method),
		"tenant_id", attempt.TenantID(),
	)

	decision, err := s.authenticate(ctx, attempt)
	if err != nil {
		l.Errorw("认证过程异常",
			"action", logger.ActionLogin,
			"scenario", string(attempt.Method),
			"error", err.Error(),
		)
		return nil, err
	}

	if !decision.OK {
		l.Warnw("认证失败",
			"action", logger.ActionLogin,
			"scenario", string(attempt.Method),
			"err_code", string(decision.ErrCode),
			"credential_id", decision.CredentialID.String(),
			"result", logger.ResultFailed,
		)
		return nil, s.failureTranslator.Translate(decision.ErrCode)
	}

	ensurePrincipalTenantID(decision.Principal)

	l.Debugw("认证成功，开始颁发令牌",
		"action", logger.ActionLogin,
		"scenario", string(attempt.Method),
		"user_id", decision.Principal.UserID.String(),
		"account_id", decision.Principal.AccountID.String(),
		"tenant_id", decision.Principal.TenantID.String(),
		"amr", decision.Principal.AMR,
		"claims", decision.Principal.Claims,
		"should_rotate", decision.ShouldRotate,
	)

	tokenPair, err := s.tokenIssuer.IssueToken(ctx, decision.Principal)
	if err != nil {
		l.Errorw("令牌颁发失败",
			"action", logger.ActionLogin,
			"user_id", decision.Principal.UserID.String(),
			"account_id", decision.Principal.AccountID.String(),
			"tenant_id", decision.Principal.TenantID.String(),
			"amr", decision.Principal.AMR,
			"claims", decision.Principal.Claims,
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return nil, perrors.WithCode(code.ErrInvalidArgument, "failed to issue token: %v", err)
	}

	l.Debugw("登录完成",
		"action", logger.ActionLogin,
		"user_id", decision.Principal.UserID.String(),
		"account_id", decision.Principal.AccountID.String(),
		"tenant_id", decision.Principal.TenantID,
		"result", logger.ResultSuccess,
	)

	return &SignInResult{
		Principal: decision.Principal,
		TokenPair: tokenPair,
		UserID:    decision.Principal.UserID,
		AccountID: decision.Principal.AccountID,
		TenantID:  decision.Principal.TenantID,
	}, nil
}

func (s *SignIn) selectMethod(ctx context.Context, cmd SignInCommand) (SignInAttempt, error) {
	selector := s.methodSelector
	if selector == nil {
		selector = newDefaultMethodSelector(newDefaultSignInAdapterCatalog(signInAdapterDeps{}))
	}
	attempt, err := selector.Select(ctx, cmd)
	if err != nil {
		return SignInAttempt{}, err
	}
	logger.L(ctx).Debugw("认证准备完成",
		"action", logger.ActionLogin,
		"scenario", string(attempt.Method),
		"tenant_id", attempt.TenantID(),
	)
	return attempt, nil
}

func (s *SignIn) authenticate(ctx context.Context, attempt SignInAttempt) (authentication.AuthDecision, error) {
	switch adapter := attempt.Adapter.(type) {
	case DomainProofAdapter:
		if s.domainAuthenticator == nil {
			return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "authenticator is not initialized")
		}
		proof, err := adapter.PrepareProof(ctx, attempt.Payload)
		if err != nil {
			return authentication.AuthDecision{}, err
		}
		return s.domainAuthenticator.Authenticate(ctx, proof)
	case BearerCompatibilityAdapter:
		return adapter.Reauthenticate(ctx, attempt.Payload)
	default:
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported sign-in adapter: %s", attempt.Method)
	}
}

// ensurePrincipalTenantID 补齐认证主体的默认 tenant_id。
// 当前默认业务租户沿用 system bootstrap / QS 约定的 org_id=1，避免登录后签出的 JWT 仍然携带 0。
func ensurePrincipalTenantID(principal *authentication.Principal) {
	if principal == nil || !principal.TenantID.IsZero() {
		return
	}
	principal.TenantID = meta.FromUint64(tenant.DefaultTenantID)
}
