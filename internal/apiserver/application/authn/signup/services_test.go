package signup

import (
	"context"
	"errors"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/uow"
	idpresolver "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
	loginidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	userDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/user"
	idpidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
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
	tx      uow.TxRepositories
	err     error
	onEnter func()
}

func (s onboardingUoWStub) WithinTx(ctx context.Context, fn func(context.Context, uow.TxRepositories) error) error {
	if s.onEnter != nil {
		s.onEnter()
	}
	if s.err != nil {
		return s.err
	}
	return fn(ctx, s.tx)
}

func TestSignupResolvesExternalIdentityBeforeOpeningTransaction(t *testing.T) {
	t.Parallel()

	resolver := &signupResolverStub{identity: newWechatMiniIdentity(t, "wx-app", "openid-1", "union-1")}
	txEntered := false
	service := NewSignupService(onboardingUoWStub{
		err: errors.New("stop before persistence"),
		onEnter: func() {
			txEntered = true
			require.Equal(t, 1, resolver.calls, "provider exchange must finish before the local transaction starts")
		},
	}, nil, resolver, nil)
	appID := "wx-app"
	jsCode := "js-code"

	_, err := service.SignUp(context.Background(), SignupRequest{
		LoginIdentity: WechatMiniLoginIdentityInput{AppID: &appID, JsCode: &jsCode},
	})

	require.Error(t, err)
	require.True(t, txEntered)
	require.Equal(t, 1, resolver.calls)
}

func TestOnboardPreservesLoginIdentityDisabledErrorCode(t *testing.T) {
	t.Parallel()

	phone, err := meta.NewPhone("13800138013")
	require.NoError(t, err)
	tenantID := meta.FromUint64(9001)
	loginID := "existing-login"
	key := mustUsernameProviderKey(t, tenantID, loginID)
	existingUser, err := userDomain.NewUser("existing", phone, userDomain.WithID(meta.FromUint64(100)))
	require.NoError(t, err)
	userRepo := &userRepoStub{
		users: map[uint64]*userDomain.User{
			existingUser.ID.Uint64(): existingUser,
		},
	}
	identityRepo := &loginIdentityRepoStub{
		byKey: map[string]*loginidentity.LoginIdentity{
			providerKey(key.Provider(), key.Realm(), key.Identifier()): {
				ID:         meta.FromUint64(20),
				UserID:     existingUser.ID,
				Provider:   key.Provider(),
				Realm:      key.Realm(),
				Identifier: key.Identifier(),
				Status:     loginidentity.StatusDisabled,
			},
		},
	}
	onboarder := &signupService{
		uow: onboardingUoWStub{tx: uow.TxRepositories{
			Users:           userRepo,
			LoginIdentities: identityRepo,
		}},
		prepareStep:             newPrepareStep(nil),
		resolveUserStep:         newResolveUserStep(userRepo),
		ensureLoginIdentityStep: newEnsureLoginIdentityStep(),
		ensureCredentialStep:    newEnsureCredentialStep(onboardingPasswordHasherStubLocal{}),
	}

	_, err = onboarder.SignUp(context.Background(), SignupRequest{
		User: SignupUserInput{
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

	key := mustUsernameProviderKey(t, meta.FromUint64(9001), "new-login")
	result, err := newResolveUserStep(userRepo).Run(
		context.Background(),
		registrationRepositories{
			Users: userRepo,
			LoginIdentities: &loginIdentityRepoStub{
				byKey: map[string]*loginidentity.LoginIdentity{},
			},
		},
		&preparedSignup{
			User: SignupUserInput{
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

	resolver := &signupResolverStub{identity: newWechatMiniIdentity(t, appID, "openid-1", "union-1")}
	prepared, err := input.prepareSignupLoginIdentity(
		context.Background(),
		loginIdentityPrepareDeps{externalIdentityResolver: resolver},
		SignupUserInput{},
	)

	require.NoError(t, err)
	require.Nil(t, input.OpenID)
	require.Nil(t, input.UnionID)
	require.NotNil(t, input.JsCode)
	require.Equal(t, "js-code", *input.JsCode)
	require.Equal(t, "openid-1", prepared.ProviderKey.Identifier())
	require.Equal(t, "union-1", prepared.ProviderKey.GlobalIdentifier())
	require.Equal(t, loginIdentitySourceProviderVerified, prepared.Source)
}

func TestWechatIdentityResolverUsesExternalIdentityResolver(t *testing.T) {
	t.Parallel()

	appID := "wx-app"
	jsCode := "js-code"
	resolver := &signupResolverStub{identity: newWechatMiniIdentity(t, appID, "openid-1", "union-1")}

	prepared, err := (WechatMiniLoginIdentityInput{
		AppID:  &appID,
		JsCode: &jsCode,
	}).prepareSignupLoginIdentity(context.Background(), loginIdentityPrepareDeps{externalIdentityResolver: resolver}, SignupUserInput{})

	require.NoError(t, err)
	require.Equal(t, "openid-1", prepared.ProviderKey.Identifier())
	require.Equal(t, "union-1", prepared.ProviderKey.GlobalIdentifier())
	require.Equal(t, loginIdentitySourceProviderVerified, prepared.Source)
	require.Equal(t, 1, resolver.calls)
	require.Equal(t, idpidentity.ProviderWechatMinip, resolver.request.Provider)
	require.Equal(t, "wx-app", resolver.request.Realm)
	require.Equal(t, "js-code", resolver.request.Code)
}

func TestWechatIdentityResolverUsesExistingOpenIDWithoutCodeExchange(t *testing.T) {
	t.Parallel()

	openID := "openid-1"
	unionID := "union-1"
	jsCode := "js-code"
	appID := "wx-app"
	resolver := &signupResolverStub{identity: newWechatMiniIdentity(t, appID, "should-not-use", "")}

	prepared, err := (WechatMiniLoginIdentityInput{
		AppID:   &appID,
		OpenID:  &openID,
		UnionID: &unionID,
		JsCode:  &jsCode,
	}).prepareSignupLoginIdentity(context.Background(), loginIdentityPrepareDeps{externalIdentityResolver: resolver}, SignupUserInput{})

	require.NoError(t, err)
	require.Equal(t, "openid-1", prepared.ProviderKey.Identifier())
	require.Equal(t, "union-1", prepared.ProviderKey.GlobalIdentifier())
	require.Equal(t, loginIdentitySourceTrustedLegacyInput, prepared.Source)
	require.Zero(t, resolver.calls)
}

func TestWechatIdentityResolverRejectsMissingOpenIDAndCodeExchangeInput(t *testing.T) {
	t.Parallel()

	_, err := (WechatMiniLoginIdentityInput{}).prepareSignupLoginIdentity(context.Background(), loginIdentityPrepareDeps{}, SignupUserInput{})

	require.Error(t, err)
}

func TestPrepareStepResolvesWechatIdentityBeforePersistenceFlow(t *testing.T) {
	t.Parallel()

	appID := " wx-app "
	jsCode := " js-code "
	resolver := &signupResolverStub{identity: newWechatMiniIdentity(t, "wx-app", "openid-1", "union-1")}
	preparer := newPrepareStep(resolver)

	prepared, err := preparer.Run(context.Background(), SignupRequest{
		LoginIdentity: WechatMiniLoginIdentityInput{
			AppID:  &appID,
			JsCode: &jsCode,
		},
	})

	require.NoError(t, err)
	require.False(t, prepared.LoginIdentity.NeedPasswordCredential)
	require.True(t, prepared.LoginIdentity.AllowUserRepair)
	require.Equal(t, loginidentity.ProviderWechatMinip, prepared.LoginIdentity.ProviderKey.Provider())
	require.Equal(t, "wx-app", prepared.LoginIdentity.ProviderKey.Realm())
	require.Equal(t, "openid-1", prepared.LoginIdentity.ProviderKey.Identifier())
	require.Equal(t, "union-1", prepared.LoginIdentity.ProviderKey.GlobalIdentifier())
	require.Equal(t, loginIdentitySourceProviderVerified, prepared.LoginIdentity.Source)
	require.Equal(t, 1, resolver.calls)
	require.Equal(t, "wx-app", resolver.request.Realm)
	require.Equal(t, "js-code", resolver.request.Code)
}

type signupResolverStub struct {
	identity idpidentity.ExternalIdentity
	err      error
	request  idpresolver.ResolveRequest
	calls    int
}

func (s *signupResolverStub) Resolve(_ context.Context, request idpresolver.ResolveRequest) (idpidentity.ExternalIdentity, error) {
	s.calls++
	s.request = request
	return s.identity, s.err
}

func newWechatMiniIdentity(t *testing.T, realm, openID, unionID string) idpidentity.ExternalIdentity {
	t.Helper()
	identifiers := make([]idpidentity.Identifier, 0, 2)
	for kind, value := range map[idpidentity.IdentifierKind]string{
		idpidentity.IdentifierOpenID:  openID,
		idpidentity.IdentifierUnionID: unionID,
	} {
		if value == "" {
			continue
		}
		identifier, err := idpidentity.NewIdentifier(kind, value)
		require.NoError(t, err)
		identifiers = append(identifiers, identifier)
	}
	identity, err := idpidentity.New(idpidentity.ProviderWechatMinip, realm, identifiers, time.Now())
	require.NoError(t, err)
	return identity
}
