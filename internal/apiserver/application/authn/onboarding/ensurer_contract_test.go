package onboarding

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestCredentialEnsurerReturnsNotRequiredWithoutPlaceholderCredential(t *testing.T) {
	t.Parallel()

	result, err := newCredentialEnsurer(onboardingPasswordHasherStubLocal{}).Ensure(
		context.Background(),
		nil,
		&LoginIdentityEnsureResult{Identity: &loginidentity.LoginIdentity{ID: meta.FromUint64(10)}},
		&preparedOnboarding{},
	)

	require.NoError(t, err)
	require.Equal(t, CredentialNotRequired, result.Status)
	require.Nil(t, result.Credential)
	require.False(t, result.HasCredential())
}

func TestLoginIdentityEnsurerRejectsProviderKeyOwnedByAnotherUser(t *testing.T) {
	t.Parallel()

	key := loginidentity.UsernameProviderKey(meta.FromUint64(9001), "zhangsan")
	repo := &loginIdentityRepoStub{
		byKey: map[string]*loginidentity.LoginIdentity{
			providerKey(key.Provider, key.Realm, key.Identifier): {
				ID:         meta.FromUint64(11),
				UserID:     meta.FromUint64(100),
				Provider:   key.Provider,
				Realm:      key.Realm,
				Identifier: key.Identifier,
				Status:     loginidentity.StatusActive,
			},
		},
	}

	_, err := newLoginIdentityEnsurer().Ensure(
		context.Background(),
		repo,
		&preparedOnboarding{
			LoginIdentity: preparedLoginIdentity{
				ProviderKey: key,
			},
		},
		meta.FromUint64(200),
	)

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrLoginIdentityExists))
}

func TestLoginIdentityEnsurerReusesActiveProviderKeyOwnedBySameUser(t *testing.T) {
	t.Parallel()

	key := loginidentity.UsernameProviderKey(meta.FromUint64(9001), "lisi")
	existing := &loginidentity.LoginIdentity{
		ID:         meta.FromUint64(13),
		UserID:     meta.FromUint64(100),
		Provider:   key.Provider,
		Realm:      key.Realm,
		Identifier: key.Identifier,
		Status:     loginidentity.StatusActive,
	}
	repo := &loginIdentityRepoStub{
		byKey: map[string]*loginidentity.LoginIdentity{
			providerKey(key.Provider, key.Realm, key.Identifier): existing,
		},
	}

	result, err := newLoginIdentityEnsurer().Ensure(
		context.Background(),
		repo,
		&preparedOnboarding{
			LoginIdentity: preparedLoginIdentity{
				ProviderKey: key,
			},
		},
		existing.UserID,
	)

	require.NoError(t, err)
	require.Equal(t, LoginIdentityReused, result.Status)
	require.Same(t, existing, result.Identity)
	require.Zero(t, repo.createCalls)
}

func TestLoginIdentityEnsurerRejectsInactiveExistingProviderKey(t *testing.T) {
	t.Parallel()

	key := loginidentity.UsernameProviderKey(meta.FromUint64(9001), "wangwu")
	repo := &loginIdentityRepoStub{
		byKey: map[string]*loginidentity.LoginIdentity{
			providerKey(key.Provider, key.Realm, key.Identifier): {
				ID:         meta.FromUint64(12),
				UserID:     meta.FromUint64(100),
				Provider:   key.Provider,
				Realm:      key.Realm,
				Identifier: key.Identifier,
				Status:     loginidentity.StatusDisabled,
			},
		},
	}

	_, err := newLoginIdentityEnsurer().Ensure(
		context.Background(),
		repo,
		&preparedOnboarding{
			LoginIdentity: preparedLoginIdentity{
				ProviderKey: key,
			},
		},
		meta.FromUint64(100),
	)

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrLoginIdentityDisabled))
}

type loginIdentityRepoStub struct {
	byKey       map[string]*loginidentity.LoginIdentity
	createCalls int
}

func (s *loginIdentityRepoStub) Create(context.Context, *loginidentity.LoginIdentity) error {
	s.createCalls++
	return nil
}

func (s *loginIdentityRepoStub) GetByID(context.Context, meta.ID) (*loginidentity.LoginIdentity, error) {
	return nil, nil
}

func (s *loginIdentityRepoStub) GetByProviderKey(_ context.Context, provider loginidentity.Provider, realm, identifier string) (*loginidentity.LoginIdentity, error) {
	return s.byKey[providerKey(provider, realm, identifier)], nil
}

func (s *loginIdentityRepoStub) GetByGlobalIdentifier(context.Context, loginidentity.Provider, string) (*loginidentity.LoginIdentity, error) {
	return nil, nil
}

func (s *loginIdentityRepoStub) ListByUserID(context.Context, meta.ID) ([]*loginidentity.LoginIdentity, error) {
	return nil, nil
}

func (s *loginIdentityRepoStub) UpdateStatus(context.Context, meta.ID, loginidentity.Status) error {
	return nil
}

func providerKey(provider loginidentity.Provider, realm, identifier string) string {
	return string(provider) + "|" + realm + "|" + identifier
}

type onboardingPasswordHasherStubLocal struct{}

func (onboardingPasswordHasherStubLocal) Verify(string, string) bool { return true }
func (onboardingPasswordHasherStubLocal) NeedRehash(string) bool     { return false }
func (onboardingPasswordHasherStubLocal) Hash(plaintext string) (string, error) {
	return "hash:" + plaintext, nil
}
func (onboardingPasswordHasherStubLocal) Pepper() string { return "pepper" }
