package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	credentialapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/credential"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/FangcunMount/iam/v2/pkg/tenant"
)

// SignIn 编排一次登录：选择登录方式、准备领域 proof、调用领域认证并签发令牌。
type SignIn struct {
	tokenService        tokenapp.TokenApplicationService
	methodRegistry      MethodRegistry
	proofFactory        ProofFactory
	domainAuthenticator *authentication.Authenticator
	credentialRecorder  credentialapp.Recorder
}

// Execute 执行登录
func (s *SignIn) Execute(ctx context.Context, cmd LoginCommand) (*LoginResult, error) {
	// 检查登录服务是否初始化
	if s == nil || s.tokenService == nil || s.methodRegistry == nil || s.proofFactory == nil || s.domainAuthenticator == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "login service is not initialized")
	}

	// 选择登录方式
	selection, err := s.methodRegistry.Select(ctx, cmd)
	if err != nil {
		return nil, wrapLoginStageError(err, code.ErrUnsupportedAuthMethod, "failed to select login method")
	}

	// 构建认证凭据
	credential, err := s.proofFactory.Build(ctx, selection)
	if err != nil {
		return nil, wrapLoginStageError(err, code.ErrProofBuildFailed, "failed to build credential")
	}

	// 进行认证
	decision, err := s.domainAuthenticator.Authenticate(ctx, credential)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to authenticate")
	}

	// 记录认证生命周期状态。
	if err := s.recordCredential(ctx, decision); err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to record credential")
	}

	// 认证不通过，返回认证失败错误。
	if !decision.OK {
		return nil, authFailureError(decision.Code)
	}

	// 补齐认证主体的默认租户ID
	ensurePrincipalTenantID(decision.Principal)

	// 颁发令牌
	tokenPair, err := s.tokenService.IssueToken(ctx, decision.Principal)
	if err != nil {
		return nil, perrors.WithCode(code.ErrAuthenticationFailed, "failed to issue token: %w", err)
	}

	// 返回结果
	return &SignInResult{
		Principal:       decision.Principal,
		TokenPair:       tokenPair,
		UserID:          decision.Principal.UserID,
		LoginIdentityID: decision.Principal.LoginIdentityID,
		TenantID:        decision.Principal.TenantID,
	}, nil
}

// recordCredential 记录长期 Credential 的认证生命周期状态。
func (s *SignIn) recordCredential(ctx context.Context, decision authentication.AuthDecision) error {
	// 如果 recorder 未初始化，直接返回。
	if s == nil || s.credentialRecorder == nil {
		return nil
	}
	// 记录认证生命周期状态。
	if err := s.credentialRecorder.Record(ctx, decision); err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to record credential")
	}
	return nil
}

// wrapLoginStageError 包装登录阶段错误。
func wrapLoginStageError(err error, fallbackCode int, message string) error {
	codeValue := fallbackCode
	if coder := perrors.ParseCoder(err); coder != nil && coder.Code() != 0 {
		codeValue = coder.Code()
	}
	return perrors.WrapC(err, codeValue, "%s", message)
}

// ensurePrincipalTenantID 补齐认证主体的默认租户ID
func ensurePrincipalTenantID(principal *authentication.Principal) {
	if principal == nil || !principal.TenantID.IsZero() {
		return
	}
	principal.TenantID = meta.FromUint64(tenant.DefaultTenantID)
}
