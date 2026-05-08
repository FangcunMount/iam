package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/FangcunMount/iam/v2/pkg/tenant"
)

// SignIn 编排一次登录：选择登录方式、准备领域 proof、调用领域认证并签发令牌。
type SignIn struct {
	tokenIssuer         tokenapp.Issuer
	methodRegistry      MethodRegistry
	proofFactory        ProofFactory
	domainAuthenticator *authentication.Authenticator
}

// Execute 执行登录
func (s *SignIn) Execute(ctx context.Context, cmd LoginCommand) (*LoginResult, error) {
	// 检查登录服务是否初始化
	if s == nil || s.methodRegistry == nil || s.proofFactory == nil || s.domainAuthenticator == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "login service is not initialized")
	}

	// 选择登录方式
	selection, err := s.methodRegistry.Select(ctx, cmd)
	if err != nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "failed to select login method: %w", err)
	}

	// 构建认证凭据
	credential, err := s.proofFactory.Build(ctx, selection)
	if err != nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "failed to build credential: %w", err)
	}

	// 进行认证
	decision, err := s.domainAuthenticator.Authenticate(ctx, credential)
	if err != nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "failed to authenticate: %w", err)
	}
	if !decision.OK {
		return nil, authFailureError(decision.Code)
	}

	// 补齐认证主体的默认租户ID
	ensurePrincipalTenantID(decision.Principal)

	// 颁发令牌
	tokenPair, err := s.tokenIssuer.IssueToken(ctx, decision.Principal)
	if err != nil {
		return nil, perrors.WithCode(code.ErrAuthenticationFailed, "failed to issue token: %w", err)
	}

	// 返回结果
	return &SignInResult{
		Principal: decision.Principal,
		TokenPair: tokenPair,
		UserID:    decision.Principal.UserID,
		AccountID: decision.Principal.AccountID,
		TenantID:  decision.Principal.TenantID,
	}, nil
}

// ensurePrincipalTenantID 补齐认证主体的默认租户ID
func ensurePrincipalTenantID(principal *authentication.Principal) {
	if principal == nil || !principal.TenantID.IsZero() {
		return
	}
	principal.TenantID = meta.FromUint64(tenant.DefaultTenantID)
}
