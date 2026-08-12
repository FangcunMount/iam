package method

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// Selector 是选择登录方式并解析对应 payload的接口。
type Selector interface {
	// Select 选择登录方式并解析对应 payload。
	Select(ctx context.Context, cmd LoginRequest) (LoginMethodSelection, error)
}

// Registry 是基于 AuthMethod 的登录方式注册表。
type Registry struct {
	ordered  []LoginMethod
	byMethod map[AuthMethod]LoginMethod
}

// 确保 Registry 实现了 Selector 接口。
var _ Selector = (*Registry)(nil)

// NewSelector 创建选择器。
func NewSelector(methods ...LoginMethod) (*Registry, error) {
	byCredentialKind := make(map[CredentialKind]struct{}, len(methods))
	ordered := make([]LoginMethod, 0, len(methods))
	byMethod := make(map[AuthMethod]LoginMethod, len(methods))

	for _, loginMethod := range methods {
		if loginMethod == nil {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "login method is required")
		}
		credentialKind := loginMethod.CredentialKind()
		authMethod := loginMethod.Method()
		if credentialKind == "" {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "login credential kind is required")
		}
		if authMethod == "" {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "login auth method is required")
		}
		if _, exists := byCredentialKind[credentialKind]; exists {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "duplicate login credential kind: %s", credentialKind)
		}
		if _, exists := byMethod[authMethod]; exists {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "duplicate login auth method: %s", authMethod)
		}

		byCredentialKind[credentialKind] = struct{}{}
		byMethod[authMethod] = loginMethod
		ordered = append(ordered, loginMethod)
	}

	return &Registry{
		ordered:  ordered,
		byMethod: byMethod,
	}, nil
}

// MustSelector 创建选择器，如果创建失败则panic
func MustSelector(methods ...LoginMethod) *Registry {
	selector, err := NewSelector(methods...)
	if err != nil {
		panic(err)
	}
	return selector
}

// DefaultSelector 创建默认选择器。
func DefaultSelector() Selector {
	return MustSelector(
		NewPasswordMethod(),
		NewPhoneOTPMethod(),
		NewWechatMiniMethod(),
		NewWechatScanMethod(),
		NewWecomMethod(),
	)
}

// LoginMethods 返回所有登录方式。
func (s *Registry) LoginMethods() []LoginMethod {
	if s == nil {
		return nil
	}
	methods := make([]LoginMethod, len(s.ordered))
	copy(methods, s.ordered)
	return methods
}

// Select 选择登录方式。
func (s *Registry) Select(_ context.Context, cmd LoginRequest) (LoginMethodSelection, error) {
	authMethod := AuthMethod(strings.TrimSpace(string(cmd.AuthMethod)))
	if authMethod == "" {
		return LoginMethodSelection{}, perrors.WithCode(code.ErrUnsupportedAuthMethod, "auth method is required")
	}

	loginMethod, ok := s.findMethod(authMethod)
	if !ok {
		return LoginMethodSelection{}, perrors.WithCode(code.ErrUnsupportedAuthMethod, "unsupported authentication method: %s", authMethod)
	}

	common := CommonPayloadFromLoginRequest(cmd)
	payload, err := loginMethod.BuildPayload(cmd)
	if err != nil {
		return LoginMethodSelection{}, err
	}

	return LoginMethodSelection{
		AuthMethod:     authMethod,
		CredentialKind: loginMethod.CredentialKind(),
		Common:         common,
		Payload:        payload,
	}, nil
}

// findMethod 查找登录方式
func (s *Registry) findMethod(authMethod AuthMethod) (LoginMethod, bool) {
	if s == nil {
		return nil, false
	}
	loginMethod, ok := s.byMethod[authMethod]
	return loginMethod, ok
}

// PublicAuthMethods 返回公开认证方法（全仓库唯一白名单，compatibility 与 login 门面均引用此处）。
func PublicAuthMethods() []AuthMethod {
	return []AuthMethod{
		AuthMethodPassword,
		AuthMethodPhoneOTP,
		AuthMethodWechatMini,
		AuthMethodWechatScan,
		AuthMethodWecom,
	}
}

// wireAliasWechat 是历史对外 wire 名："wechat" 即微信小程序登录（内部 wechat_mini）。
// 保留该别名是为了兼容既有客户端与 OpenAPI 契约，内部统一收敛为 AuthMethodWechatMini。
const wireAliasWechat = "wechat"

// NormalizeWireAuthMethod 将对外 wire auth_method 归一为内部 AuthMethod。
//
// 仅处理 wire 名与内部名的差异（如 "wechat" -> "wechat_mini"），不做白名单校验。
func NormalizeWireAuthMethod(raw string) AuthMethod {
	authMethod := AuthMethod(strings.TrimSpace(raw))
	if authMethod == wireAliasWechat {
		return AuthMethodWechatMini
	}
	return authMethod
}

// IsPublicAuthMethod 判断是否是公开认证方法（接受 wire 别名）。
func IsPublicAuthMethod(raw string) bool {
	authMethod := NormalizeWireAuthMethod(raw)
	for _, allowed := range PublicAuthMethods() {
		if authMethod == allowed {
			return true
		}
	}
	return false
}
