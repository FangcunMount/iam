package proof

import (
	"context"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin/method"
	idpresolver "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	idpidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestMethodProofPreparersMapPayloads(t *testing.T) {
	t.Parallel()

	passwordProof, err := NewPasswordBuilder().Build(context.Background(), method.PasswordPayload{
		Username: "alice",
		Password: "secret",
	}, method.CommonPayload{TenantID: meta.FromUint64(42)})
	require.NoError(t, err)
	password, ok := passwordProof.(*authentication.PasswordCredential)
	require.True(t, ok)
	require.Equal(t, authentication.CredentialKindPassword, password.CredentialKind())
	require.Equal(t, uint64(42), password.TenantID.Uint64())
	require.Equal(t, "alice", password.Username)

	phoneProof, err := NewPhoneOTPBuilder().Build(context.Background(), method.PhoneOTPPayload{
		PhoneE164: "+8613800138000",
		OTP:       "123456",
	}, method.CommonPayload{TenantID: meta.FromUint64(7)})
	require.NoError(t, err)
	phone, ok := phoneProof.(*authentication.PhoneOTPCredential)
	require.True(t, ok)
	require.Equal(t, authentication.CredentialKindPhoneOTP, phone.CredentialKind())
	require.Equal(t, "+8613800138000", phone.PhoneE164)
}

func TestWecomMethodMapsResolverConfigurationFailure(t *testing.T) {
	t.Parallel()

	resolver := &wecomResolverStub{err: &idpresolver.ResolutionError{
		Kind:     idpresolver.ErrorProviderConfig,
		Provider: idpidentity.ProviderWecom,
		Realm:    "corp-id",
	}}
	factory := DefaultFactory(resolver, nil)
	credential, err := factory.Build(context.Background(), wecomSelection())

	require.Nil(t, credential)
	require.Error(t, err)
	require.Equal(t, code.ErrProofBuildFailed, perrors.ParseCoder(err).Code())
	require.Equal(t, 1, resolver.calls)
}

func TestWecomMethodMapsResolverAppFailures(t *testing.T) {
	t.Parallel()

	for _, kind := range []idpresolver.ErrorKind{
		idpresolver.ErrorAppQueryFailed,
		idpresolver.ErrorAppNotFound,
		idpresolver.ErrorAppDisabled,
		idpresolver.ErrorCredentialMissing,
		idpresolver.ErrorSecretDecryptFailed,
	} {
		kind := kind
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			resolver := &wecomResolverStub{err: &idpresolver.ResolutionError{
				Kind:     kind,
				Provider: idpidentity.ProviderWecom,
				Realm:    "corp-id",
			}}
			credential, err := DefaultFactory(resolver, nil).Build(context.Background(), wecomSelection())

			require.Nil(t, credential)
			require.Error(t, err)
			require.Equal(t, code.ErrProofBuildFailed, perrors.ParseCoder(err).Code())
		})
	}
}

func TestWecomMethodUsesResolvedIdentityAndAuthenticates(t *testing.T) {
	t.Parallel()

	loginIdentityID := meta.FromUint64(1001)
	userID := meta.FromUint64(2002)
	identityRepo := &wecomLoginIdentityRepoStub{
		lookup: &authentication.LoginIdentityLookup{
			LoginIdentityID: loginIdentityID,
			UserID:          userID,
			Provider:        loginidentity.ProviderWecom,
			Realm:           "corp-id",
			Identifier:      "wecom-user-id",
			Status:          loginidentity.StatusActive,
		},
	}
	resolver := &wecomResolverStub{identity: newWecomIdentity(t, "corp-id", "wecom-user-id", "open-user-id")}
	auth := authentication.NewAuthenticator(authentication.NewOAuthWeChatComAuthStrategyWithLoginIdentity(identityRepo))
	factory := DefaultFactory(resolver, nil)

	credential, err := factory.Build(context.Background(), wecomSelection())
	require.NoError(t, err)
	decision, err := auth.Authenticate(context.Background(), credential)

	require.NoError(t, err)
	require.True(t, decision.OK)
	require.Equal(t, loginIdentityID, decision.Principal.LoginIdentityID)
	require.Equal(t, userID, decision.Principal.UserID)
	require.True(t, decision.CredentialID.IsZero())
	require.Equal(t, 1, resolver.calls)
	require.Equal(t, idpidentity.ProviderWecom, resolver.request.Provider)
	require.Equal(t, "corp-id", resolver.request.Realm)
	require.Equal(t, "auth-code", resolver.request.Code)
	require.Equal(t, loginidentity.ProviderWecom, identityRepo.provider)
	require.Equal(t, "corp-id", identityRepo.realm)
	require.Equal(t, "wecom-user-id", identityRepo.identifier)
}

func wecomSelection() method.LoginMethodSelection {
	return method.LoginMethodSelection{
		CredentialKind: method.CredentialKindWecom,
		Common:         method.CommonPayload{TenantID: meta.FromUint64(1)},
		Payload: method.WecomPayload{
			CorpID: "corp-id",
			Code:   "auth-code",
		},
	}
}

type wecomResolverStub struct {
	identity idpidentity.ExternalIdentity
	err      error
	request  idpresolver.ResolveRequest
	calls    int
}

func (s *wecomResolverStub) Resolve(_ context.Context, request idpresolver.ResolveRequest) (idpidentity.ExternalIdentity, error) {
	s.calls++
	s.request = request
	return s.identity, s.err
}

func newWecomIdentity(t *testing.T, realm, userID, openUserID string) idpidentity.ExternalIdentity {
	t.Helper()
	identifiers := make([]idpidentity.Identifier, 0, 2)
	for kind, value := range map[idpidentity.IdentifierKind]string{
		idpidentity.IdentifierUserID:     userID,
		idpidentity.IdentifierOpenUserID: openUserID,
	} {
		identifier, err := idpidentity.NewIdentifier(kind, value)
		require.NoError(t, err)
		identifiers = append(identifiers, identifier)
	}
	identity, err := idpidentity.New(idpidentity.ProviderWecom, realm, identifiers, time.Now())
	require.NoError(t, err)
	return identity
}

type wecomLoginIdentityRepoStub struct {
	lookup     *authentication.LoginIdentityLookup
	provider   loginidentity.Provider
	realm      string
	identifier string
}

func (s *wecomLoginIdentityRepoStub) FindUsernameIdentity(context.Context, meta.ID, string) (*authentication.LoginIdentityLookup, error) {
	return nil, nil
}

func (s *wecomLoginIdentityRepoStub) FindLoginIdentityByProviderKey(_ context.Context, provider loginidentity.Provider, realm, identifier string) (*authentication.LoginIdentityLookup, error) {
	s.provider = provider
	s.realm = realm
	s.identifier = identifier
	if s.lookup == nil || s.lookup.Provider != provider || s.lookup.Realm != realm || s.lookup.Identifier != identifier {
		return nil, nil
	}
	return s.lookup, nil
}

func (s *wecomLoginIdentityRepoStub) FindLoginIdentityByGlobalIdentifier(context.Context, loginidentity.Provider, string) (*authentication.LoginIdentityLookup, error) {
	return nil, nil
}

func (s *wecomLoginIdentityRepoStub) IsLoginIdentityActive(context.Context, meta.ID) (bool, error) {
	return true, nil
}
