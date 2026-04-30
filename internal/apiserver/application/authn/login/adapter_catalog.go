package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	tokenapp "github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	idpPort "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

type SignInAdapter interface {
	Kind() SignInKind
	AuthType() AuthType
	TryLegacy(SignInCommand, methodPayloadCommon) (MethodPayload, bool)
	BuildExplicit(SignInCommand, methodPayloadCommon) (MethodPayload, error)
}

type DomainProofAdapter interface {
	SignInAdapter
	PrepareProof(ctx context.Context, payload MethodPayload) (authentication.AuthCredential, error)
}

type BearerCompatibilityAdapter interface {
	SignInAdapter
	Reauthenticate(ctx context.Context, payload MethodPayload) (authentication.AuthDecision, error)
}

type SignInAdapterCatalog struct {
	ordered    []SignInAdapter
	byAuthType map[AuthType]SignInAdapter
}

func newSignInAdapterCatalog(adapters ...SignInAdapter) (*SignInAdapterCatalog, error) {
	byKind := make(map[SignInKind]struct{}, len(adapters))
	byAuthType := make(map[AuthType]SignInAdapter, len(adapters))
	ordered := make([]SignInAdapter, 0, len(adapters))

	for _, adapter := range adapters {
		if adapter == nil {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "sign-in adapter is required")
		}
		kind := adapter.Kind()
		authType := adapter.AuthType()
		if kind == "" {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "sign-in method kind is required")
		}
		if authType == "" {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "sign-in method auth type is required")
		}
		if _, exists := byKind[kind]; exists {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "duplicate sign-in method kind: %s", kind)
		}
		if _, exists := byAuthType[authType]; exists {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "duplicate sign-in method auth type: %s", authType)
		}
		byKind[kind] = struct{}{}
		byAuthType[authType] = adapter
		ordered = append(ordered, adapter)
	}

	return &SignInAdapterCatalog{
		ordered:    ordered,
		byAuthType: byAuthType,
	}, nil
}

func mustSignInAdapterCatalog(adapters ...SignInAdapter) *SignInAdapterCatalog {
	catalog, err := newSignInAdapterCatalog(adapters...)
	if err != nil {
		panic(err)
	}
	return catalog
}

func (c *SignInAdapterCatalog) adapters() []SignInAdapter {
	if c == nil {
		return nil
	}
	adapters := make([]SignInAdapter, len(c.ordered))
	copy(adapters, c.ordered)
	return adapters
}

func (c *SignInAdapterCatalog) findAuthType(authType AuthType) (SignInAdapter, bool) {
	if c == nil {
		return nil, false
	}
	adapter, ok := c.byAuthType[authType]
	return adapter, ok
}

type signInAdapterDeps struct {
	wechatAppQuerier idpPort.Repository
	secretVault      idpPort.SecretVault
	wecomConfig      WecomConfig
	tokenVerifier    tokenapp.Verifier
}

func newDefaultSignInAdapterCatalog(deps signInAdapterDeps) *SignInAdapterCatalog {
	return mustSignInAdapterCatalog(
		newPasswordAdapter(),
		newPhoneOTPAdapter(),
		newWechatMiniAdapter(deps.wechatAppQuerier, deps.secretVault),
		newWecomAdapter(deps.wechatAppQuerier, deps.secretVault, deps.wecomConfig),
		newBearerAdapter(deps.tokenVerifier),
	)
}
