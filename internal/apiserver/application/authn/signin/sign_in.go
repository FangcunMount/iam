package signin

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	admissionapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/authfailure"
	authnexternal "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/principal"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin/method"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// SignIn 登录
type SignIn struct {
	deps Dependencies // 依赖
}

// New 创建 SignIn 用例。
// 参数：deps 依赖
// 返回：SignIn 用例
// 职责：创建 SignIn 用例，返回 SignIn 用例
func New(deps Dependencies) *SignIn {
	return &SignIn{deps: deps}
}

// Execute 执行登录。
func (s *SignIn) Execute(ctx context.Context, cmd method.LoginRequest) (*Result, error) {
	// 确保依赖已准备好
	if err := s.ensureReady(); err != nil {
		return nil, err
	}

	// 构建领域认证凭据
	credential, err := s.buildCredential(ctx, cmd)
	if err != nil {
		return nil, err
	}

	// 领域认证
	decision, err := s.authenticate(ctx, credential)
	if err != nil {
		return nil, err
	}

	// 记录认证结果
	if err := s.recordCredential(ctx, decision); err != nil {
		return nil, err
	}

	// 如果认证不通过，返回认证失败
	if !decision.OK {
		return nil, authfailure.Error(decision.Code)
	}

	if decision.Principal == nil {
		return nil, perrors.WithCode(code.ErrAuthenticationFailed, "authentication principal is missing")
	}
	if err := admissionapp.Require(
		ctx,
		s.deps.AdmissionPolicy,
		decision.Principal.UserID,
		decision.Principal.LoginIdentityID,
	); err != nil {
		return nil, err
	}

	// 签发 TokenPair
	return s.issueTokenPair(ctx, decision.Principal)
}

// ensureReady 确保依赖已准备好
// 参数：s SignIn 用例
// 返回：错误
// 职责：确保依赖已准备好，返回错误
func (s *SignIn) ensureReady() error {
	d := s.deps
	if s == nil || d.SessionEstablisher == nil || d.MethodRegistry == nil || d.ProofFactory == nil || d.Authenticator == nil || d.AdmissionPolicy == nil {
		return perrors.WithCode(code.ErrInvalidArgument, "login service is not initialized")
	}
	return nil
}

// buildCredential 构建领域认证凭据
// 参数：ctx 上下文, cmd 登录命令
// 返回：领域认证凭据, 错误
// 职责：构建领域认证凭据，返回领域认证凭据
func (s *SignIn) buildCredential(ctx context.Context, cmd method.LoginRequest) (authentication.AuthCredential, error) {
	// 选择登录方式
	selection, err := s.deps.MethodRegistry.Select(ctx, cmd)
	if err != nil {
		return nil, wrapStageError(err, code.ErrUnsupportedAuthMethod, "failed to select login method")
	}

	// 构建领域认证凭据
	credential, err := s.deps.ProofFactory.Build(ctx, selection)
	if err != nil {
		if cause, ok := authnexternal.AuthenticationCause(err); ok {
			return nil, perrors.WrapC(cause, code.ErrInternalServerError, "failed to authenticate")
		}
		return nil, wrapStageError(err, code.ErrProofBuildFailed, "failed to build credential")
	}
	return credential, nil
}

// authenticate 领域认证
// 参数：ctx 上下文, credential 领域认证凭据
// 返回：认证决策, 错误
// 职责：领域认证，返回认证决策
func (s *SignIn) authenticate(ctx context.Context, credential authentication.AuthCredential) (authentication.AuthDecision, error) {
	// 领域认证
	decision, err := s.deps.Authenticator.Authenticate(ctx, credential)
	if err != nil {
		return authentication.AuthDecision{}, perrors.WrapC(err, code.ErrInternalServerError, "failed to authenticate")
	}
	// 返回认证决策
	return decision, nil
}

// issueTokenPair 签发 TokenPair
// 参数：ctx 上下文, p 认证主体
// 返回：登录结果, 错误
// 职责：签发 TokenPair，返回登录结果
func (s *SignIn) issueTokenPair(ctx context.Context, p *authentication.Principal) (*Result, error) {
	// 确保 token 上下文已设置
	principal.EnsureTokenContext(p)

	// 签发 TokenPair
	tokenPair, err := s.deps.SessionEstablisher.EstablishSession(ctx, p)
	if err != nil {
		return nil, perrors.WithCode(code.ErrAuthenticationFailed, "failed to issue token: %w", err)
	}

	// 由认证主体构造登录结果
	return ResultFromPrincipal(p, tokenPair), nil
}

// recordCredential 记录认证结果
// 参数：ctx 上下文, decision 认证决策
// 返回：错误
// 职责：记录认证结果，返回错误
func (s *SignIn) recordCredential(ctx context.Context, decision authentication.AuthDecision) error {
	if s.deps.CredentialRecorder == nil {
		return nil
	}
	if err := s.deps.CredentialRecorder.Record(ctx, decision); err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to record credential")
	}
	return nil
}

// wrapStageError 包装阶段错误
// 参数：err 错误, fallbackCode 默认错误码, message 错误消息
// 返回：错误
// 职责：包装阶段错误，返回错误
func wrapStageError(err error, fallbackCode int, message string) error {
	codeValue := fallbackCode
	if coder := perrors.ParseCoder(err); coder != nil && coder.Code() != 0 {
		codeValue = coder.Code()
	}
	return perrors.WrapC(err, codeValue, "%s", message)
}
