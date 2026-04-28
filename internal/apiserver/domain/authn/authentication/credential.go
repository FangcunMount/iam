package authentication

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

type AuthCredential interface {
	Scenario() Scenario
}

// CredentialBuilder 构建领域凭据的工厂函数
type CredentialBuilder func(input AuthInput) (AuthCredential, error)

var credentialBuilders = make(map[Scenario]CredentialBuilder)

// RegisterCredentialBuilder 注册凭据构造器
func RegisterCredentialBuilder(kind Scenario, builder CredentialBuilder) {
	if _, exists := credentialBuilders[kind]; exists {
		return
	}
	credentialBuilders[kind] = builder
}

func getCredentialBuilder(kind Scenario) (CredentialBuilder, error) {
	builder, ok := credentialBuilders[kind]
	if !ok {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "unsupported auth scenario: %s", kind)
	}
	return builder, nil
}
