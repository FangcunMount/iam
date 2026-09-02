package signup

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestCredentialEnsurerReturnsNotRequiredWithoutPlaceholderCredential(t *testing.T) {
	t.Parallel()

	result, err := newEnsureCredentialStep(onboardingPasswordHasherStubLocal{}).Run(
		context.Background(),
		nil,
		&ensureLoginIdentityStepResult{Identity: &loginidentity.LoginIdentity{ID: meta.FromUint64(10)}},
		&preparedSignup{},
	)

	require.NoError(t, err)
	require.Equal(t, CredentialNotRequired, result.Status)
	require.Nil(t, result.Credential)
	require.False(t, result.HasCredential())
}

func TestLoginIdentityEnsurerRejectsProviderKeyOwnedByAnotherUser(t *testing.T) {
	t.Parallel()

	key := mustUsernameProviderKey(t, meta.FromUint64(9001), "zhangsan")
	repo := &loginIdentityRepoStub{
		byKey: map[string]*loginidentity.LoginIdentity{
			providerKey(key.Provider(), key.Realm(), key.Identifier()): {
				ID:         meta.FromUint64(11),
				UserID:     meta.FromUint64(100),
				Provider:   key.Provider(),
				Realm:      key.Realm(),
				Identifier: key.Identifier(),
				Status:     loginidentity.StatusActive,
			},
		},
	}

	_, err := newEnsureLoginIdentityStep().Run(
		context.Background(),
		repo,
		&preparedSignup{
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

	key := mustUsernameProviderKey(t, meta.FromUint64(9001), "lisi")
	existing := &loginidentity.LoginIdentity{
		ID:         meta.FromUint64(13),
		UserID:     meta.FromUint64(100),
		Provider:   key.Provider(),
		Realm:      key.Realm(),
		Identifier: key.Identifier(),
		Status:     loginidentity.StatusActive,
		Profile:    map[string]string{"nickname": "original"},
		Meta:       map[string]string{"source": "original-source"},
	}
	repo := &loginIdentityRepoStub{
		byKey: map[string]*loginidentity.LoginIdentity{
			providerKey(key.Provider(), key.Realm(), key.Identifier()): existing,
		},
	}

	result, err := newEnsureLoginIdentityStep().Run(
		context.Background(),
		repo,
		&preparedSignup{
			LoginIdentity: preparedLoginIdentity{
				ProviderKey: key,
				Profile:     map[string]string{"nickname": "must-not-overwrite"},
				Meta:        map[string]string{"source": "must-not-overwrite"},
			},
		},
		existing.UserID,
	)

	require.NoError(t, err)
	require.Equal(t, LoginIdentityReused, result.Status)
	require.Same(t, existing, result.Identity)
	require.Equal(t, "original", result.Identity.Profile["nickname"])
	require.Equal(t, "original-source", result.Identity.Meta["source"])
	require.Zero(t, repo.createCalls)
}

func TestLoginIdentityEnsurerPersistsMockConsumerProfileAndMetaOnCreate(t *testing.T) {
	t.Parallel()

	key, err := loginidentity.NewMockConsumerProviderKey("daily-mock@example.com")
	require.NoError(t, err)
	repo := &loginIdentityRepoStub{byKey: map[string]*loginidentity.LoginIdentity{}}
	result, err := newEnsureLoginIdentityStep().Run(
		context.Background(),
		repo,
		&preparedSignup{LoginIdentity: preparedLoginIdentity{
			ProviderKey: key,
			Profile:     map[string]string{"nickname": "每日模拟用户"},
			Meta: map[string]string{
				"source":   "daily_simulation",
				"run_date": "2026-08-02",
			},
		}},
		meta.FromUint64(100),
	)

	require.NoError(t, err)
	require.Equal(t, LoginIdentityCreated, result.Status)
	require.Equal(t, 1, repo.createCalls)
	require.NotNil(t, repo.created)
	require.Equal(t, "每日模拟用户", repo.created.Profile["nickname"])
	require.Equal(t, "daily_simulation", repo.created.Meta["source"])
	require.Equal(t, "2026-08-02", repo.created.Meta["run_date"])
}

func TestLoginIdentityEnsurerRejectsInactiveExistingProviderKey(t *testing.T) {
	t.Parallel()

	key := mustUsernameProviderKey(t, meta.FromUint64(9001), "wangwu")
	repo := &loginIdentityRepoStub{
		byKey: map[string]*loginidentity.LoginIdentity{
			providerKey(key.Provider(), key.Realm(), key.Identifier()): {
				ID:         meta.FromUint64(12),
				UserID:     meta.FromUint64(100),
				Provider:   key.Provider(),
				Realm:      key.Realm(),
				Identifier: key.Identifier(),
				Status:     loginidentity.StatusDisabled,
			},
		},
	}

	_, err := newEnsureLoginIdentityStep().Run(
		context.Background(),
		repo,
		&preparedSignup{
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
	created     *loginidentity.LoginIdentity
}

func (s *loginIdentityRepoStub) Create(_ context.Context, identity *loginidentity.LoginIdentity) error {
	s.createCalls++
	s.created = identity
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

func providerKey(provider loginidentity.Provider, realm, identifier string) string {
	return string(provider) + "|" + realm + "|" + identifier
}

func mustUsernameProviderKey(t *testing.T, tenantID meta.ID, username string) loginidentity.ProviderKey {
	t.Helper()
	key, err := loginidentity.NewUsernameProviderKey(tenantID, username)
	require.NoError(t, err)
	return key
}

type onboardingPasswordHasherStubLocal struct{}

func (onboardingPasswordHasherStubLocal) Verify(string, string) bool { return true }
func (onboardingPasswordHasherStubLocal) NeedRehash(string) bool     { return false }
func (onboardingPasswordHasherStubLocal) Hash(plaintext string) (string, error) {
	return "hash:" + plaintext, nil
}
func (onboardingPasswordHasherStubLocal) Pepper() string { return "pepper" }
