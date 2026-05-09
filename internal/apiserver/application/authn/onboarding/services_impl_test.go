package onboarding

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
	accountDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/account"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	idpWechatApp "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type userRepoStub struct {
	users map[uint64]*userDomain.User
}

func (s *userRepoStub) Create(_ context.Context, user *userDomain.User) error {
	if s.users == nil {
		s.users = make(map[uint64]*userDomain.User)
	}
	if _, exists := s.users[user.ID.Uint64()]; exists {
		return perrors.WithCode(code.ErrUserAlreadyExists, "user already exists")
	}
	s.users[user.ID.Uint64()] = user
	return nil
}

func (s *userRepoStub) FindByID(_ context.Context, id meta.ID) (*userDomain.User, error) {
	if s.users == nil {
		return nil, perrors.WithCode(code.ErrUserNotFound, "user not found")
	}
	user, ok := s.users[id.Uint64()]
	if !ok {
		return nil, perrors.WithCode(code.ErrUserNotFound, "user not found")
	}
	return user, nil
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

type accountRepoStub struct {
	byUniqueID      map[string]*accountDomain.Account
	byExternalIDApp map[string]*accountDomain.Account
}

func (s *accountRepoStub) Create(context.Context, *accountDomain.Account) error { return nil }
func (s *accountRepoStub) UpdateUniqueID(context.Context, meta.ID, accountDomain.UnionID) error {
	return nil
}
func (s *accountRepoStub) UpdateStatus(context.Context, meta.ID, accountDomain.AccountStatus) error {
	return nil
}
func (s *accountRepoStub) UpdateProfile(context.Context, meta.ID, map[string]string) error {
	return nil
}
func (s *accountRepoStub) UpdateMeta(context.Context, meta.ID, map[string]string) error { return nil }
func (s *accountRepoStub) GetByID(context.Context, meta.ID) (*accountDomain.Account, error) {
	return nil, perrors.WithCode(code.ErrNotFoundAccount, "account not found")
}
func (s *accountRepoStub) GetByUniqueID(_ context.Context, uniqueID accountDomain.UnionID) (*accountDomain.Account, error) {
	if account, ok := s.byUniqueID[string(uniqueID)]; ok {
		return account, nil
	}
	return nil, perrors.WithCode(code.ErrNotFoundAccount, "account not found")
}
func (s *accountRepoStub) GetByExternalIDAppId(_ context.Context, externalID accountDomain.ExternalID, appID accountDomain.AppId) (*accountDomain.Account, error) {
	if account, ok := s.byExternalIDApp[string(externalID)+"|"+string(appID)]; ok {
		return account, nil
	}
	return nil, perrors.WithCode(code.ErrNotFoundAccount, "account not found")
}

func TestUserResolverRepairsDanglingWechatAccountUser(t *testing.T) {
	t.Parallel()

	resolver := newUserResolver(nil)
	userRepo := &userRepoStub{users: make(map[uint64]*userDomain.User)}
	accountUserID := meta.FromUint64(615206334492586542)
	accountRepo := &accountRepoStub{
		byUniqueID: map[string]*accountDomain.Account{
			"union-1": accountDomain.NewAccount(accountUserID, accountDomain.TypeWcMinip, accountDomain.ExternalID("openid@app"), accountDomain.WithID(meta.FromUint64(1))),
		},
	}
	email, err := meta.NewEmail("clack@fangcunmount.com")
	require.NoError(t, err)
	unionID := "union-1"

	req := &NormalizedOnboardingRequest{
		OnboardingRequest: OnboardingRequest{
			Name:           "clack",
			Email:          email,
			AccountType:    accountDomain.TypeWcMinip,
			CredentialType: CredTypeWechat,
			WechatUnionID:  &unionID,
			Profile: map[string]string{
				"nickname": "clack",
			},
		},
		Plan: OnboardingPlan{
			Scenario:        OnboardWechatMini,
			AccountType:     accountDomain.TypeWcMinip,
			CredentialType:  CredTypeWechat,
			AllowUserRepair: true,
		},
		strategy: defaultStrategies.byScenario[OnboardWechatMini],
	}

	result, err := resolver.Resolve(context.Background(), registrationRepositories{
		Users:    userRepo,
		Accounts: accountRepo,
	}, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, UserRepaired, result.Status)
	require.Equal(t, MatchedByWechatUnionID, result.MatchedBy)
	require.False(t, result.IsNewUser())
	require.Equal(t, accountUserID.Uint64(), result.User.ID.Uint64())
	require.Equal(t, "clack", result.User.Name)
	require.Equal(t, "clack", result.User.Nickname)
	require.Equal(t, "clack@fangcunmount.com", result.User.Email.String())

	stored, err := userRepo.FindByID(context.Background(), accountUserID)
	require.NoError(t, err)
	require.Equal(t, accountUserID.Uint64(), stored.ID.Uint64())
}

func TestPrepareWechatIdentityDoesNotMutateOriginalRequest(t *testing.T) {
	t.Parallel()

	appID := "wx-app"
	jsCode := "js-code"
	req := OnboardingRequest{
		AccountType:  accountDomain.TypeWcMinip,
		WechatAppID:  &appID,
		WechatJsCode: &jsCode,
	}

	prepared := prepareWechatIdentity(req, wechatIdentity{
		OpenID:  "openid-1",
		UnionID: "union-1",
	})

	require.Nil(t, req.WechatOpenID)
	require.Nil(t, req.WechatUnionID)
	require.NotNil(t, req.WechatJsCode)
	require.Equal(t, "js-code", *req.WechatJsCode)

	require.NotNil(t, prepared.WechatOpenID)
	require.Equal(t, "openid-1", *prepared.WechatOpenID)
	require.NotNil(t, prepared.WechatUnionID)
	require.Equal(t, "union-1", *prepared.WechatUnionID)
	require.Nil(t, prepared.WechatJsCode)
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

	identity, err := resolver.ResolveMiniProgram(context.Background(), OnboardingRequest{
		AccountType:  accountDomain.TypeWcMinip,
		WechatAppID:  &appID,
		WechatJsCode: &jsCode,
	})

	require.NoError(t, err)
	require.Equal(t, "openid-1", identity.OpenID)
	require.Equal(t, "union-1", identity.UnionID)
	require.Equal(t, "wx-app", idp.appID)
	require.Equal(t, "app-secret", idp.appSecret)
	require.Equal(t, "js-code", idp.jsCode)
}

func TestRequestNormalizerResolvesWechatIdentityOutsidePersistenceFlow(t *testing.T) {
	t.Parallel()

	appID := " wx-app "
	jsCode := " js-code "
	idp := &onboardingIDPStub{openID: "openid-1", unionID: "union-1"}
	normalizer := newRequestNormalizer(newWechatIdentityResolver(
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

	normalized, err := normalizer.Normalize(context.Background(), OnboardingRequest{
		AccountType:    accountDomain.TypeWcMinip,
		CredentialType: CredTypeWechat,
		WechatAppID:    &appID,
		WechatJsCode:   &jsCode,
	})

	require.NoError(t, err)
	require.Equal(t, OnboardWechatMini, normalized.Plan.Scenario)
	require.Equal(t, accountDomain.TypeWcMinip, normalized.Plan.AccountType)
	require.Equal(t, CredTypeWechat, normalized.Plan.CredentialType)
	require.Equal(t, "openid-1", *normalized.WechatOpenID)
	require.Equal(t, "union-1", *normalized.WechatUnionID)
	require.Nil(t, normalized.WechatJsCode)
	require.Equal(t, "wx-app", idp.appID)
	require.Equal(t, "js-code", idp.jsCode)
}

func TestBuildPlanRejectsInvalidAccountCredentialCombination(t *testing.T) {
	t.Parallel()

	_, err := BuildPlan(OnboardingRequest{
		AccountType:    accountDomain.TypeOpera,
		CredentialType: CredTypeWechat,
	})

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestBuildPlanSelectsStrategyByAccountType(t *testing.T) {
	t.Parallel()

	plan, err := BuildPlan(OnboardingRequest{
		AccountType:    accountDomain.TypeWcMinip,
		CredentialType: CredTypeWechat,
	})

	require.NoError(t, err)
	require.Equal(t, OnboardWechatMini, plan.Scenario)
	require.Equal(t, accountDomain.TypeWcMinip, plan.AccountType)
	require.Equal(t, CredTypeWechat, plan.CredentialType)
	require.True(t, plan.AllowUserRepair)
	require.True(t, plan.AllowCredentialReuse)
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
