package login

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/authfailure"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/principal"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// Reauthenticate 编排已有 access token 的再验证。
type Reauthenticate struct {
	reAuthenticator ReAuthenticator
}

// NewReauthenticate 创建再验证用例。
func NewReauthenticate(reAuthenticator ReAuthenticator) *Reauthenticate {
	return &Reauthenticate{reAuthenticator: reAuthenticator}
}

// Execute 验证 access token 并返回 Principal。
func (r *Reauthenticate) Execute(ctx context.Context, tokenValue string) (*authentication.Principal, error) {
	// 确保再验证器已准备好
	if err := r.ensureReady(); err != nil {
		return nil, err
	}

	// 验证 access token 值是否有效
	tokenValue, err := validateAccessTokenValue(tokenValue)
	if err != nil {
		return nil, err
	}

	// 再验证 access token 是否仍然有效
	decision, err := r.reAuthenticator.Reauthenticate(ctx, tokenValue)
	if err != nil {
		return nil, err
	}

	// 如果再验证不通过，返回认证失败
	if !decision.OK {
		return nil, authfailure.Error(decision.Code)
	}

	// 确保 token 上下文已设置
	principal.EnsureTokenContext(decision.Principal)
	return decision.Principal, nil
}

// ensureReady 确保再验证器已准备好
func (r *Reauthenticate) ensureReady() error {
	if r == nil || r.reAuthenticator == nil {
		return perrors.WithCode(code.ErrInvalidArgument, "re-authenticator is not initialized")
	}
	return nil
}

// validateAccessTokenValue 验证 access token 值是否有效
func validateAccessTokenValue(tokenValue string) (string, error) {
	tokenValue = strings.TrimSpace(tokenValue)
	if tokenValue == "" {
		return "", perrors.WithCode(code.ErrInvalidArgument, "access token is required")
	}
	return tokenValue, nil
}

// ==================================================
// ================== Internal Types ==================
// ==================================================

// tokenReAuthenticator 是 token 再验证器。
type tokenReAuthenticator struct {
	tokenVerifier TokenVerifier
}

// 确保 tokenReAuthenticator 实现了 ReAuthenticator 接口。
var _ ReAuthenticator = (*tokenReAuthenticator)(nil)

// NewTokenReAuthenticator 将 token 验证能力适配为 ReAuthenticator。
func NewTokenReAuthenticator(tokenVerifier TokenVerifier) ReAuthenticator {
	if tokenVerifier == nil {
		return nil
	}
	return &tokenReAuthenticator{tokenVerifier: tokenVerifier}
}

// Reauthenticate 再验证 access token 是否仍然有效
// 参数：ctx 上下文, tokenValue access token 值
// 返回：认证决策, 错误
// 职责：验证 access token 是否仍然有效，返回认证决策
func (r *tokenReAuthenticator) Reauthenticate(ctx context.Context, tokenValue string) (authentication.AuthDecision, error) {
	if r == nil || r.tokenVerifier == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "token verifier is not initialized")
	}

	// 验证 access token 是否仍然有效
	result, err := r.tokenVerifier.VerifyToken(ctx, tokenapp.VerifyTokenRequest{AccessToken: tokenValue})
	if err != nil {
		return authentication.AuthDecision{}, err
	}

	// 如果验证不通过，返回认证失败
	if result == nil || !result.Valid || result.Claims == nil {
		codeValue := code.ErrTokenInvalid
		if result != nil && result.FailureCode != 0 {
			codeValue = result.FailureCode
		}

		// 如果验证失败是再验证失败，返回认证失败
		if isReauthenticationFailureCode(codeValue) {
			return authentication.AuthDecision{OK: false, Code: codeValue}, nil
		}

		// 如果验证失败是其他原因，返回认证失败
		return authentication.AuthDecision{OK: false, Code: code.ErrTokenInvalid}, nil
	}

	// 如果验证通过，返回认证成功
	return authentication.AuthDecision{
		OK:        true,
		Principal: principalFromTokenClaims(result.Claims),
	}, nil
}

// isReauthenticationFailureCode 判断是否是再验证失败代码
func isReauthenticationFailureCode(codeValue int) bool {
	switch codeValue {
	case code.ErrTokenInvalid,
		code.ErrExpired,
		code.ErrUserBlocked,
		code.ErrUserInactive,
		code.ErrLoginIdentityDisabled,
		code.ErrCredentialLocked:
		return true
	default:
		return false
	}
}

// principalFromTokenClaims 从 token claims 构造认证主体
func principalFromTokenClaims(claims *tokenapp.TokenClaims) *authentication.Principal {
	if claims == nil {
		return nil
	}
	merged := tokenAttributeClaims(claims.Attributes)
	if merged == nil {
		merged = make(map[string]any)
	}
	if domain := claims.TenantDomain; domain != "" {
		merged["tenant_domain"] = domain
	}
	if !claims.OrgID.IsZero() {
		merged["org_id"] = claims.OrgID.String()
	}
	return &authentication.Principal{
		UserID:          claims.UserID,
		LoginIdentityID: claims.LoginIdentityID,
		SessionID:       claims.SessionID,
		AMR:             cloneStrings(claims.AMR),
		Claims:          merged,
	}
}

// tokenAttributeClaims 从 token attributes 构造认证主体 claims
func tokenAttributeClaims(attributes map[string]string) map[string]any {
	if len(attributes) == 0 {
		return nil
	}
	claims := make(map[string]any, len(attributes))
	for key, value := range attributes {
		claims[key] = value
	}
	return claims
}

// cloneStrings 克隆字符串切片
func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
