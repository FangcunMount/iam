package reauth

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// tokenReAuthenticator 是 token 再验证器。
type tokenReAuthenticator struct {
	tokenVerifier TokenVerifier
}

// TokenVerifier 是再验证需要的 token 能力。
type TokenVerifier interface {
	VerifyToken(ctx context.Context, req tokenapp.VerifyTokenRequest) (*tokenapp.TokenVerifyResult, error)
}

// 确保 tokenReAuthenticator 实现了 ReAuthenticator 接口。
var _ ReAuthenticator = (*tokenReAuthenticator)(nil)

// NewTokenReAuthenticator 将已有 token verifier 适配为登录态再验证用例依赖。
func NewTokenReAuthenticator(tokenVerifier TokenVerifier) ReAuthenticator {
	if tokenVerifier == nil {
		return nil
	}
	return &tokenReAuthenticator{tokenVerifier: tokenVerifier}
}

// Reauthenticate 执行再验证，验证 access token 是否仍然有效。
func (r *tokenReAuthenticator) Reauthenticate(ctx context.Context, tokenValue string) (authentication.AuthDecision, error) {
	if r == nil || r.tokenVerifier == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "token verifier is not initialized")
	}

	// 验证 access token 是否仍然有效
	result, err := r.tokenVerifier.VerifyToken(ctx, tokenapp.VerifyTokenRequest{AccessToken: tokenValue})
	if err != nil {
		return authentication.AuthDecision{}, err
	}
	if result == nil || !result.Valid || result.Claims == nil {
		codeValue := code.ErrTokenInvalid
		if result != nil && result.FailureCode != 0 {
			codeValue = result.FailureCode
		}
		if isReauthenticationFailureCode(codeValue) {
			return authentication.AuthDecision{OK: false, Code: codeValue}, nil
		}
		return authentication.AuthDecision{OK: false, Code: code.ErrTokenInvalid}, nil
	}

	// 构造认证决策
	return authentication.AuthDecision{
		OK:        true,
		Principal: principalFromTokenClaims(result.Claims),
	}, nil
}

// isReauthenticationFailureCode 判断是否是再验证失败代码。
func isReauthenticationFailureCode(codeValue int) bool {
	switch codeValue {
	case code.ErrTokenInvalid,
		code.ErrExpired,
		code.ErrUserBlocked,
		code.ErrUserInactive,
		code.ErrCredentialDisabled,
		code.ErrCredentialLocked:
		return true
	default:
		return false
	}
}

// principalFromTokenClaims 从 token claims 构造认证主体。
func principalFromTokenClaims(claims *tokenapp.TokenClaims) *authentication.Principal {
	if claims == nil {
		return nil
	}
	return &authentication.Principal{
		UserID:          claims.UserID,
		LoginIdentityID: claims.LoginIdentityID,
		TenantID:        claims.TenantID,
		SessionID:       claims.SessionID,
		AMR:             cloneStrings(claims.AMR),
		Claims:          tokenAttributeClaims(claims.Attributes),
	}
}

// tokenAttributeClaims 从 token attributes 构造认证主体 claims。
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

// cloneStrings 克隆字符串切片。
func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
