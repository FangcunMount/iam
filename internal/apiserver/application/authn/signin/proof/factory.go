package proof

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/method"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	idpPort "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// WecomConfig 企业微信配置
type WecomConfig struct {
	AgentID string
}

// Builder 证明构造器
type Builder interface {
	CredentialKind() method.CredentialKind
	Build(ctx context.Context, payload method.Payload, common method.CommonPayload) (authentication.AuthCredential, error)
}

// CredentialFactory 将登录方式选择结果构造成领域认证凭据。
type CredentialFactory interface {
	Build(ctx context.Context, selection method.LoginMethodSelection) (authentication.AuthCredential, error)
}

// Factory 证明工厂
type Factory struct {
	builders map[method.CredentialKind]Builder
}

var _ CredentialFactory = (*Factory)(nil)

// NewFactory 创建证明工厂
func NewFactory(builders ...Builder) (*Factory, error) {
	byKind := make(map[method.CredentialKind]Builder, len(builders))
	for _, builder := range builders {
		if builder == nil {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "proof builder is required")
		}
		kind := builder.CredentialKind()
		if kind == "" {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "proof builder credential kind is required")
		}
		if _, exists := byKind[kind]; exists {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "duplicate proof builder credential kind: %s", kind)
		}
		byKind[kind] = builder
	}
	return &Factory{builders: byKind}, nil
}

// MustFactory 创建证明工厂，如果创建失败则panic
func MustFactory(builders ...Builder) *Factory {
	factory, err := NewFactory(builders...)
	if err != nil {
		panic(err)
	}
	return factory
}

// DefaultFactory 创建默认证明工厂
func DefaultFactory(repo idpPort.Repository, vault idpPort.SecretVault, wecomConfig WecomConfig) *Factory {
	return MustFactory(
		NewPasswordBuilder(),
		NewPhoneOTPBuilder(),
		newWechatBuilder(repo, vault),
		newWecomBuilder(repo, vault, wecomConfig),
	)
}

// Build 构建证明
func (f *Factory) Build(ctx context.Context, selection method.LoginMethodSelection) (authentication.AuthCredential, error) {
	if selection.Payload == nil {
		return nil, perrors.WithCode(code.ErrProofBuildFailed, "method payload is required")
	}
	builder := f.builderFor(selection.CredentialKind)
	if builder == nil {
		return nil, perrors.WithCode(code.ErrProofBuildFailed, "unsupported credential kind: %s", selection.CredentialKind)
	}
	return builder.Build(ctx, selection.Payload, selection.Common)
}

// builderFor 获取证明构造器
func (f *Factory) builderFor(kind method.CredentialKind) Builder {
	if f == nil {
		return nil
	}
	return f.builders[kind]
}
