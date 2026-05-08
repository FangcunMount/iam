package login

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// Reauthenticate 编排已有 access token 的再验证。
type Reauthenticate struct {
	reAuthenticator ReAuthenticator
}

// Execute 执行再验证，验证 access token 是否仍然有效。
func (r *Reauthenticate) Execute(ctx context.Context, tokenValue string) (*AuthResult, error) {
	if r == nil || r.reAuthenticator == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "re-authenticator is not initialized")
	}

	// 验证 token value 是否为空
	tokenValue = strings.TrimSpace(tokenValue)
	if tokenValue == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "access token is required")
	}

	// 执行再验证
	decision, err := r.reAuthenticator.Reauthenticate(ctx, tokenValue)
	if err != nil {
		return nil, err
	}

	// 验证再验证结果
	if !decision.OK {
		return nil, authFailureError(decision.Code)
	}

	// 补齐认证主体的默认租户ID
	ensurePrincipalTenantID(decision.Principal)

	// 返回认证结果
	return authResultFromPrincipal(decision.Principal), nil
}

// authResultFromPrincipal 从认证主体构造认证结果。
func authResultFromPrincipal(principal *authentication.Principal) *AuthResult {
	if principal == nil {
		return &AuthResult{}
	}
	return &AuthResult{
		Principal: principal,
		UserID:    principal.UserID,
		AccountID: principal.AccountID,
		TenantID:  principal.TenantID,
	}
}
