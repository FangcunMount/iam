package onboarding

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/uow"
	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	idpWechatApp "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type userRepoStub struct {
	users            map[uint64]*userDomain.User
	createCalls      int
	findByPhoneCalls int
}

func (s *userRepoStub) Create(_ context.Context, user *userDomain.User) error {
	if s.users == nil {
		s.users = make(map[uint64]*userDomain.User)
	}
	s.createCalls++
	s.users[user.ID.Uint64()] = user
	return nil
}

func (s *userRepoStub) FindByID(_ context.Context, id meta.ID) (*userDomain.User, error) {
	if s.users == nil {
		return nil, nil
	}
	return s.users[id.Uint64()], nil
}

func (s *userRepoStub) FindByIDs(_ context.Context, ids []meta.ID) (map[meta.ID]*userDomain.User, error) {
	out := make(map[meta.ID]*userDomain.User, len(ids))
	for _, id := range ids {
		if s.users == nil {
			continue
		}
		if user := s.users[id.Uint64()]; user != nil {
			out[id] = user
		}
	}
	return out, nil
}

func (s *userRepoStub) FindByPhone(_ context.Context, phone meta.Phone) (*userDomain.User, error) {
	s.findByPhoneCalls++
	for _, user := range s.users {
		if user.Phone.String() == phone.String() {
			return user, nil
		}
	}
	return nil, nil
}

func (s *userRepoStub) Update(_ context.Context, user *userDomain.User) error {
	if s.users == nil {
		s.users = make(map[uint64]*userDomain.User)
	}
	s.users[user.ID.Uint64()] = user
	return nil
}

type onboardingUoWStub struct {
	tx  uow.TxRepositories
	err error
}

func (s onboardingUoWStub) WithinTx(ctx context.Context, fn func(context.Context, uow.TxRepositories) error) error {
	if s.err != nil {
		return s.err
	}
	return fn(ctx, s.tx)
}

func TestOnboardPreservesLoginIdentityDisabledErrorCode(t *testing.T) {
	t.Parallel()

	phone, err := meta.NewPhone("13800138013")
	require.NoError(t, err)
	tenantID := meta.FromUint64(9001)
	loginID := "existing-login"
	key := loginidentity.UsernameProviderKey(tenantID, loginID)
	existingUser, err := userDomain.NewUser("existing", phone, userDomain.WithID(meta.FromUint64(100)))
	require.NoError(t, err)
	userRepo := &userRepoStub{
		users: map[uint64]*userDomain.User{
			existingUser.ID.Uint64(): existingUser,
		},
	}
	identityRepo := &loginIdentityRepoStub{
		byKey: map[string]*loginidentity.LoginIdentity{
			providerKey(key.Provider, key.Realm, key.Identifier): {
				ID:         meta.FromUint64(20),
				UserID:     existingUser.ID,
				Provider:   key.Provider,
				Realm:      key.Realm,
				Identifier: key.Identifier,
				Status:     loginidentity.StatusDisabled,
			},
		},
	}
	onboarder := &loginIdentityOnboarder{
		uow: onboardingUoWStub{tx: uow.TxRepositories{
			Users:           userRepo,
			LoginIdentities: identityRepo,
		}},
		preparer:             newRequestPreparer(nil),
		userResolver:         newUserResolver(userRepo),
		loginIdentityEnsurer: newLoginIdentityEnsurer(),
		credentialEnsurer:    newCredentialEnsurer(onboardingPasswordHasherStubLocal{}),
	}

	_, err = onboarder.Onboard(context.Background(), OnboardingRequest{
		User: OnboardingUserInput{
			Name:  "existing",
			Phone: phone,
		},
		LoginIdentity: UsernameLoginIdentityInput{
			Username:      loginID,
			RealmTenantID: tenantID,
		},
	})

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrLoginIdentityDisabled))
	require.False(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestUserResolverDoesNotReuseUserByPhoneWithoutLoginIdentity(t *testing.T) {
	t.Parallel()

	phone, err := meta.NewPhone("13800138010")
	require.NoError(t, err)
	existingUser, err := userDomain.NewUser("existing", phone, userDomain.WithID(meta.FromUint64(100)))
	require.NoError(t, err)
	userRepo := &userRepoStub{
		users: map[uint64]*userDomain.User{
			existingUser.ID.Uint64(): existingUser,
		},
	}

	key := loginidentity.UsernameProviderKey(meta.FromUint64(9001), "new-login")
	result, err := newUserResolver(userRepo).Resolve(
		context.Background(),
		registrationRepositories{
			Users: userRepo,
			LoginIdentities: &loginIdentityRepoStub{
				byKey: map[string]*loginidentity.LoginIdentity{},
			},
		},
		&preparedOnboarding{
			User: OnboardingUserInput{
				Name:  "new user",
				Phone: phone,
			},
			LoginIdentity: preparedLoginIdentity{
				ProviderKey: key,
			},
		},
	)

	require.NoError(t, err)
	require.Equal(t, UserCreated, result.Status)
	require.Equal(t, MatchedByNone, result.MatchedBy)
	require.NotEqual(t, existingUser.ID, result.User.ID)
	require.Zero(t, userRepo.findByPhoneCalls)
	require.Equal(t, 1, userRepo.createCalls)
}

func TestWechatMiniInputDoesNotMutateOriginalRequest(t *testing.T) {
	t.Parallel()

	appID := "wx-app"
	jsCode := "js-code"
	input := WechatMiniLoginIdentityInput{
		AppID:  &appID,
		JsCode: &jsCode,
	}

	prepared := input.withResolvedIdentity(wechatIdentity{OpenID: "openid-1", UnionID: "union-1"})

	require.Nil(t, input.OpenID)
	require.Nil(t, input.UnionID)
	require.NotNil(t, input.JsCode)
	require.Equal(t, "js-code", *input.JsCode)

	require.NotNil(t, prepared.OpenID)
	require.Equal(t, "openid-1", *prepared.OpenID)
	require.NotNil(t, prepared.UnionID)
	require.Equal(t, "union-1", *prepared.UnionID)
	require.Nil(t, prepared.JsCode)
}

func TestWechatIdentityResolverUsesAppConfigAndExchangesCode(t *testing.T) {
	t.Parallel()

	appID := "wx-app"
	jsCode := "js-code"
	idp := &onboardingIDPStub{openID: "openid-1", unionID: "union-1"}
	resolver := newWechatIdentityResolver(
		idp,
		&onboardingWechatAppRepoStub{
			app: &idpWechatApp.WechatApp{
				AppID:  appID,
				Status: idpWechatApp.StatusEnabled,
				Cred: &idpWechatApp.Credentials{
					Auth: &idpWechatApp.AuthSecret{AppSecretCipher: []byte("cipher")},
				},
			},
		},
		onboardingSecretVaultStub{plaintext: "app-secret"},
	)

	identity, err := resolver.ResolveMiniProgram(context.Background(), WechatMiniLoginIdentityInput{
		AppID:  &appID,
		JsCode: &jsCode,
	})

	require.NoError(t, err)
	require.Equal(t, "openid-1", identity.OpenID)
	require.Equal(t, "union-1", identity.UnionID)
	require.Equal(t, "wx-app", idp.appID)
	require.Equal(t, "app-secret", idp.appSecret)
	require.Equal(t, "js-code", idp.jsCode)
}

func TestWechatIdentityResolverUsesExistingOpenIDWithoutCodeExchange(t *testing.T) {
	t.Parallel()

	openID := "openid-1"
	unionID := "union-1"
	jsCode := "js-code"
	idp := &onboardingIDPStub{openID: "should-not-use"}
	resolver := newWechatIdentityResolver(
		idp,
		&onboardingWechatAppRepoStub{},
		onboardingSecretVaultStub{},
	)

	identity, err := resolver.ResolveMiniProgram(context.Background(), WechatMiniLoginIdentityInput{
		OpenID:  &openID,
		UnionID: &unionID,
		JsCode:  &jsCode,
	})

	require.NoError(t, err)
	require.Equal(t, "openid-1", identity.OpenID)
	require.Equal(t, "union-1", identity.UnionID)
	require.Empty(t, idp.appID)
	require.Empty(t, idp.jsCode)
}

func TestWechatIdentityResolverRejectsMissingOpenIDAndCodeExchangeInput(t *testing.T) {
	t.Parallel()

	resolver := newWechatIdentityResolver(
		&onboardingIDPStub{},
		&onboardingWechatAppRepoStub{},
		onboardingSecretVaultStub{},
	)

	_, err := resolver.ResolveMiniProgram(context.Background(), WechatMiniLoginIdentityInput{})

	require.Error(t, err)
}

func TestRequestPreparerResolvesWechatIdentityBeforePersistenceFlow(t *testing.T) {
	t.Parallel()

	appID := " wx-app "
	jsCode := " js-code "
	idp := &onboardingIDPStub{openID: "openid-1", unionID: "union-1"}
	preparer := newRequestPreparer(newWechatIdentityResolver(
		idp,
		&onboardingWechatAppRepoStub{
			app: &idpWechatApp.WechatApp{
				AppID:  "wx-app",
				Status: idpWechatApp.StatusEnabled,
				Cred: &idpWechatApp.Credentials{
					Auth: &idpWechatApp.AuthSecret{AppSecretCipher: []byte("cipher")},
				},
			},
		},
		onboardingSecretVaultStub{plaintext: "app-secret"},
	))

	prepared, err := preparer.Prepare(context.Background(), OnboardingRequest{
		LoginIdentity: WechatMiniLoginIdentityInput{
			AppID:  &appID,
			JsCode: &jsCode,
		},
	})

	require.NoError(t, err)
	require.False(t, prepared.LoginIdentity.NeedPasswordCredential)
	require.True(t, prepared.LoginIdentity.AllowUserRepair)
	require.Equal(t, loginidentity.ProviderWechatMinip, prepared.LoginIdentity.ProviderKey.Provider)
	require.Equal(t, "wx-app", prepared.LoginIdentity.ProviderKey.Realm)
	require.Equal(t, "openid-1", prepared.LoginIdentity.ProviderKey.Identifier)
	require.Equal(t, "union-1", prepared.LoginIdentity.ProviderKey.GlobalIdentifier)
	require.Equal(t, "wx-app", idp.appID)
	require.Equal(t, "js-code", idp.jsCode)
}

type onboardingWechatAppRepoStub struct {
	app *idpWechatApp.WechatApp
	err error
}

func (s *onboardingWechatAppRepoStub) Create(context.Context, *idpWechatApp.WechatApp) error {
	return nil
}

func (s *onboardingWechatAppRepoStub) GetByID(context.Context, idutil.ID) (*idpWechatApp.WechatApp, error) {
	return nil, nil
}

func (s *onboardingWechatAppRepoStub) GetByAppID(context.Context, string) (*idpWechatApp.WechatApp, error) {
	return s.app, s.err
}

func (s *onboardingWechatAppRepoStub) List(context.Context, idpWechatApp.ListFilter) ([]*idpWechatApp.WechatApp, error) {
	return nil, nil
}

func (s *onboardingWechatAppRepoStub) Update(context.Context, *idpWechatApp.WechatApp) error {
	return nil
}

type onboardingSecretVaultStub struct {
	plaintext string
	err       error
}

func (s onboardingSecretVaultStub) Encrypt(context.Context, []byte) ([]byte, error) {
	return nil, nil
}

func (s onboardingSecretVaultStub) Decrypt(context.Context, []byte) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []byte(s.plaintext), nil
}

func (s onboardingSecretVaultStub) Sign(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

type onboardingIDPStub struct {
	appID     string
	appSecret string
	jsCode    string
	openID    string
	unionID   string
}

func (s *onboardingIDPStub) ExchangeWxMinipCode(_ context.Context, appID, appSecret, jsCode string) (string, string, error) {
	s.appID = appID
	s.appSecret = appSecret
	s.jsCode = jsCode
	return s.openID, s.unionID, nil
}

func (s *onboardingIDPStub) ExchangeWecomCode(context.Context, string, string, string, string) (string, string, error) {
	return "", "", nil
}
