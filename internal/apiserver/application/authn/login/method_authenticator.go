package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	tokenapp "github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	idpPort "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// MethodAuthenticator 执行某一种认证场景。
type MethodAuthenticator interface {
	Authenticate(ctx context.Context, selected SelectedMethod) (authentication.AuthDecision, error)
}

type methodAuthenticatorRouter struct {
	byMethod map[MethodKind]MethodAuthenticator
}

func newMethodAuthenticatorRouter(
	authenticator *authentication.Authenticator,
	tokenVerifier tokenapp.Verifier,
	wechatAppQuerier idpPort.Repository,
	secretVault idpPort.SecretVault,
) *methodAuthenticatorRouter {
	return &methodAuthenticatorRouter{
		byMethod: map[MethodKind]MethodAuthenticator{
			MethodPassword: &domainMethodAuthenticator{
				authenticator: authenticator,
				buildProof:    buildPasswordProof,
			},
			MethodPhoneOTP: &domainMethodAuthenticator{
				authenticator: authenticator,
				buildProof:    buildPhoneOTPProof,
			},
			MethodWechatMini: &wechatMethodAuthenticator{
				authenticator:    authenticator,
				wechatAppQuerier: wechatAppQuerier,
				secretVault:      secretVault,
			},
			MethodWecom: &domainMethodAuthenticator{
				authenticator: authenticator,
				buildProof:    buildWecomProof,
			},
			MethodBearerToken: &bearerMethodAuthenticator{
				tokenVerifier: tokenVerifier,
			},
		},
	}
}

func (r *methodAuthenticatorRouter) Authenticate(ctx context.Context, selected SelectedMethod) (authentication.AuthDecision, error) {
	if r == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "authenticator router is not initialized")
	}
	authenticator := r.byMethod[selected.Method]
	if authenticator == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication scenario: %s", selected.Method)
	}
	return authenticator.Authenticate(ctx, selected)
}

type domainMethodAuthenticator struct {
	authenticator *authentication.Authenticator
	buildProof    func(MethodPayload) (authentication.AuthCredential, error)
}

func (a *domainMethodAuthenticator) Authenticate(ctx context.Context, selected SelectedMethod) (authentication.AuthDecision, error) {
	if a == nil || a.authenticator == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "authenticator is not initialized")
	}
	if a.buildProof == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "credential builder is not initialized")
	}
	proof, err := a.buildProof(selected.Payload)
	if err != nil {
		return authentication.AuthDecision{}, err
	}
	return a.authenticator.Authenticate(ctx, proof)
}
