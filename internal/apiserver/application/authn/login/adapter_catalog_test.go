package login

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

type fakeCatalogPayload struct {
	methodPayloadCommon
	value string
}

func (fakeCatalogPayload) methodPayload() {}

type fakeCatalogCredential struct {
	credentialType credDomain.CredentialType
}

func (c fakeCatalogCredential) CredentialType() credDomain.CredentialType {
	return c.credentialType
}

type fakeCatalogAdapter struct {
	kind     SignInKind
	authType AuthType
	legacy   func(SignInCommand, methodPayloadCommon) (MethodPayload, bool)
	explicit func(SignInCommand, methodPayloadCommon) (MethodPayload, error)
}

func (a fakeCatalogAdapter) Kind() SignInKind {
	return a.kind
}

func (a fakeCatalogAdapter) AuthType() AuthType {
	return a.authType
}

func (a fakeCatalogAdapter) TryLegacy(cmd SignInCommand, common methodPayloadCommon) (MethodPayload, bool) {
	if a.legacy == nil {
		return nil, false
	}
	return a.legacy(cmd, common)
}

func (a fakeCatalogAdapter) BuildExplicit(cmd SignInCommand, common methodPayloadCommon) (MethodPayload, error) {
	if a.explicit == nil {
		return nil, nil
	}
	return a.explicit(cmd, common)
}

func (a fakeCatalogAdapter) PrepareProof(context.Context, MethodPayload) (authentication.AuthCredential, error) {
	return fakeCatalogCredential{credentialType: credDomain.CredentialType(a.kind)}, nil
}

func TestSignInAdapterCatalogRejectsDuplicateKindAndAuthType(t *testing.T) {
	t.Parallel()

	_, err := newSignInAdapterCatalog(
		fakeCatalogAdapter{kind: "fake", authType: "fake_a"},
		fakeCatalogAdapter{kind: "fake", authType: "fake_b"},
	)
	require.Error(t, err)
	require.Equal(t, code.ErrInvalidArgument, perrors.ParseCoder(err).Code())

	_, err = newSignInAdapterCatalog(
		fakeCatalogAdapter{kind: "fake_a", authType: "fake"},
		fakeCatalogAdapter{kind: "fake_b", authType: "fake"},
	)
	require.Error(t, err)
	require.Equal(t, code.ErrInvalidArgument, perrors.ParseCoder(err).Code())
}

func TestMethodSelectorUsesCatalogDefinitions(t *testing.T) {
	t.Parallel()

	fakeAuthType := AuthType("fake")
	fakeMethod := SignInKind("fake_method")
	catalog := mustSignInAdapterCatalog(fakeCatalogAdapter{
		kind:     fakeMethod,
		authType: fakeAuthType,
		legacy: func(req SignInCommand, common methodPayloadCommon) (MethodPayload, bool) {
			if req.Username == nil || *req.Username != "legacy-fake" {
				return nil, false
			}
			return fakeCatalogPayload{methodPayloadCommon: common, value: "legacy"}, true
		},
		explicit: func(SignInCommand, methodPayloadCommon) (MethodPayload, error) {
			return fakeCatalogPayload{value: "explicit"}, nil
		},
	})
	selector := newDefaultMethodSelector(catalog)

	username := "legacy-fake"
	legacy, err := selector.Select(context.Background(), LoginRequest{Username: &username})
	require.NoError(t, err)
	require.Equal(t, fakeMethod, legacy.Method)
	require.Equal(t, "legacy", legacy.Payload.(fakeCatalogPayload).value)

	explicit, err := selector.Select(context.Background(), LoginRequest{
		SelectionMode: SignInSelectionExplicit,
		AuthType:      fakeAuthType,
	})
	require.NoError(t, err)
	require.Equal(t, fakeMethod, explicit.Method)
	require.Equal(t, "explicit", explicit.Payload.(fakeCatalogPayload).value)

	adapter, ok := explicit.Adapter.(DomainProofAdapter)
	require.True(t, ok)
	credential, err := adapter.PrepareProof(context.Background(), explicit.Payload)
	require.NoError(t, err)
	require.Equal(t, credDomain.CredentialType(fakeMethod), credential.CredentialType())
}
