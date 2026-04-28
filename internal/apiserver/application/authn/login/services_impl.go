package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	tokenDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/token"
	idpPort "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/FangcunMount/iam/pkg/tenant"
)

type loginApplicationService struct {
	tokenIssuer          tokenDomain.Issuer
	tokenRefresher       tokenDomain.Refresher
	scenarioSelector     ScenarioSelector
	methodAuthenticators MethodAuthenticator
}

var _ LoginApplicationService = (*loginApplicationService)(nil)

func NewLoginApplicationService(
	tokenIssuer tokenDomain.Issuer,
	tokenRefresher tokenDomain.Refresher,
	authenticater *authentication.Authenticater,
	wechatAppQuerier idpPort.Repository,
	secretVault idpPort.SecretVault,
) LoginApplicationService {
	return &loginApplicationService{
		tokenIssuer:          tokenIssuer,
		tokenRefresher:       tokenRefresher,
		scenarioSelector:     newDefaultScenarioSelector(),
		methodAuthenticators: newMethodAuthenticatorRouter(authenticater, wechatAppQuerier, secretVault),
	}
}

func (s *loginApplicationService) Login(ctx context.Context, req LoginRequest) (*LoginResult, error) {
	l := logger.L(ctx)

	selected, err := s.selectScenario(ctx, req)
	if err != nil {
		l.Warnw("认证准备失败",
			"action", logger.ActionLogin,
			"error", err.Error(),
		)
		return nil, err
	}

	l.Debugw("开始认证流程",
		"action", logger.ActionLogin,
		"scenario", string(selected.Scenario),
		"tenant_id", selected.Input.TenantID,
	)

	decision, err := s.authenticateMethod(ctx, selected)
	if err != nil {
		l.Errorw("认证过程异常",
			"action", logger.ActionLogin,
			"scenario", string(selected.Scenario),
			"error", err.Error(),
		)
		return nil, err
	}

	if !decision.OK {
		l.Warnw("认证失败",
			"action", logger.ActionLogin,
			"scenario", string(selected.Scenario),
			"err_code", string(decision.ErrCode),
			"credential_id", decision.CredentialID.String(),
			"result", logger.ResultFailed,
		)
		return nil, s.convertAuthError(decision.ErrCode)
	}

	ensurePrincipalTenantID(decision.Principal)

	l.Debugw("认证成功，开始颁发令牌",
		"action", logger.ActionLogin,
		"scenario", string(selected.Scenario),
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

	return &LoginResult{
		Principal: decision.Principal,
		TokenPair: tokenPair,
		UserID:    decision.Principal.UserID,
		AccountID: decision.Principal.AccountID,
		TenantID:  decision.Principal.TenantID,
	}, nil
}

// ensurePrincipalTenantID 补齐认证主体的默认 tenant_id。
// 当前默认业务租户沿用 system bootstrap / QS 约定的 org_id=1，避免登录后签出的 JWT 仍然携带 0。
func ensurePrincipalTenantID(principal *authentication.Principal) {
	if principal == nil || !principal.TenantID.IsZero() {
		return
	}
	principal.TenantID = meta.FromUint64(tenant.DefaultTenantID)
}

// Logout 登出接口 - 撤销令牌
func (s *loginApplicationService) Logout(ctx context.Context, req LogoutRequest) error {
	l := logger.L(ctx)

	// 至少需要提供一个令牌
	if (req.AccessToken == nil || *req.AccessToken == "") &&
		(req.RefreshToken == nil || *req.RefreshToken == "") {
		l.Warnw("登出请求缺少令牌",
			"action", logger.ActionLogout,
			"result", logger.ResultFailed,
		)
		return perrors.WithCode(code.ErrInvalidArgument, "either access_token or refresh_token is required")
	}

	if req.RefreshToken != nil && *req.RefreshToken != "" {
		if err := s.tokenRefresher.RevokeRefreshToken(ctx, *req.RefreshToken); err != nil {
			l.Errorw("撤销刷新令牌失败",
				"action", logger.ActionLogout,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return perrors.WithCode(code.ErrInvalidArgument, "failed to revoke refresh token: %v", err)
		}
		l.Debugw("刷新令牌已撤销",
			"action", logger.ActionLogout,
			"result", logger.ResultSuccess,
		)
	}

	if req.AccessToken != nil && *req.AccessToken != "" {
		if err := s.tokenIssuer.RevokeAccessToken(ctx, *req.AccessToken); err != nil {
			l.Errorw("撤销访问令牌失败",
				"action", logger.ActionLogout,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return perrors.WithCode(code.ErrInvalidArgument, "failed to revoke access token: %v", err)
		}
		l.Debugw("访问令牌已撤销",
			"action", logger.ActionLogout,
			"result", logger.ResultSuccess,
		)
	}

	l.Debugw("当前登录会话已退出",
		"action", logger.ActionLogout,
		"resource", "session",
		"result", logger.ResultSuccess,
	)
	return nil
}

func (s *loginApplicationService) convertAuthError(errCode authentication.ErrCode) error {
	switch errCode {
	case authentication.ErrInvalidCredential:
		return perrors.WithCode(code.ErrPasswordIncorrect, "invalid credentials")
	case authentication.ErrOTPMissingOrExpiry:
		return perrors.WithCode(code.ErrOTPInvalid, "OTP is invalid or expired")
	case authentication.ErrNoBinding:
		return perrors.WithCode(code.ErrNoBinding, "no account binding found")
	case authentication.ErrLocked:
		return perrors.WithCode(code.ErrCredentialLocked, "account is locked")
	case authentication.ErrDisabled:
		return perrors.WithCode(code.ErrCredentialDisabled, "account is disabled")
	case authentication.ErrIDPExchangeFailed:
		return perrors.WithCode(code.ErrIDPExchangeFailed, "failed to exchange with identity provider")
	case authentication.ErrStateMismatch:
		return perrors.WithCode(code.ErrStateMismatch, "state parameter mismatch")
	default:
		return perrors.WithCode(code.ErrAuthenticationFailed, "authentication failed")
	}
}

func (s *loginApplicationService) selectScenario(ctx context.Context, req LoginRequest) (SelectedScenario, error) {
	selector := s.scenarioSelector
	if selector == nil {
		selector = newDefaultScenarioSelector()
	}
	selected, err := selector.Select(ctx, req)
	if err != nil {
		return SelectedScenario{}, err
	}
	logger.L(ctx).Debugw("认证准备完成",
		"action", logger.ActionLogin,
		"scenario", string(selected.Scenario),
		"tenant_id", selected.Input.TenantID,
	)
	return selected, nil
}

func (s *loginApplicationService) authenticateMethod(ctx context.Context, selected SelectedScenario) (authentication.AuthDecision, error) {
	if s.methodAuthenticators == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "method authenticator is not initialized")
	}
	return s.methodAuthenticators.Authenticate(ctx, selected)
}
