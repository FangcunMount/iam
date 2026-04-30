package login

import (
	"context"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// SignInKind 是应用层一次登录尝试的内部标识。
// 普通账号登录直接使用 domain authentication.Scenario 的字符串值；jwt_token 仅作为兼容登录标识存在于 application/transport。
type SignInKind string

// SignInAttempt 是应用层完成 method selection 后的一次登录尝试。
type SignInAttempt struct {
	Method  SignInKind
	Adapter SignInAdapter
	Payload MethodPayload
}

func (m SignInAttempt) TenantID() meta.ID {
	if m.Payload == nil {
		return meta.ZeroID
	}
	return m.Payload.commonPayload().TenantID
}

// MethodSelector 将登录命令转换为一次登录尝试。
type MethodSelector interface {
	Select(ctx context.Context, cmd SignInCommand) (SignInAttempt, error)
}

type defaultMethodSelector struct {
	legacy   legacyMethodSelector
	explicit explicitMethodSelector
}

func newDefaultMethodSelector(catalog *SignInAdapterCatalog) MethodSelector {
	if catalog == nil {
		catalog = newDefaultSignInAdapterCatalog(signInAdapterDeps{})
	}
	return defaultMethodSelector{
		legacy:   legacyMethodSelector{catalog: catalog},
		explicit: explicitMethodSelector{catalog: catalog},
	}
}

func (s defaultMethodSelector) Select(ctx context.Context, cmd SignInCommand) (SignInAttempt, error) {
	if cmd.SelectionMode == SignInSelectionExplicit {
		return s.explicit.Select(ctx, cmd)
	}
	return s.legacy.Select(ctx, cmd)
}
