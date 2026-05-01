package onboarding

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
	accountdomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/account"
	idpwechatapp "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	userdomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type userRepoStub struct {
	users map[uint64]*userdomain.User
}

func (s *userRepoStub) Create(_ context.Context, user *userdomain.User) error {
	if s.users == nil {
		s.users = make(map[uint64]*userdomain.User)
	}
	if _, exists := s.users[user.ID.Uint64()]; exists {
		return perrors.WithCode(code.ErrUserAlreadyExists, "user already exists")
	}
	s.users[user.ID.Uint64()] = user
	return nil
}

func (s *userRepoStub) FindByID(_ context.Context, id meta.ID) (*userdomain.User, error) {
	if s.users == nil {
		return nil, perrors.WithCode(code.ErrUserNotFound, "user not found")
	}
	user, ok := s.users[id.Uint64()]
	if !ok {
		return nil, perrors.WithCode(code.ErrUserNotFound, "user not found")
	}
	return user, nil
}

func (s *userRepoStub) FindByPhone(_ context.Context, phone meta.Phone) (*userdomain.User, error) {
	for _, user := range s.users {
		if user.Phone.String() == phone.String() {
			return user, nil
		}
	}
	return nil, perrors.WithCode(code.ErrUserNotFound, "user not found")
}

func (s *userRepoStub) Update(_ context.Context, user *userdomain.User) error {
	if s.users == nil {
		s.users = make(map[uint64]*userdomain.User)
	}
	s.users[user.ID.Uint64()] = user
	return nil
}

type accountRepoStub struct {
	byUniqueID      map[string]*accountdomain.Account
	byExternalIDApp map[string]*accountdomain.Account
}

func (s *accountRepoStub) Create(context.Context, *accountdomain.Account) error { return nil }
func (s *accountRepoStub) UpdateUniqueID(context.Context, meta.ID, accountdomain.UnionID) error {
	return nil
}
func (s *accountRepoStub) UpdateStatus(context.Context, meta.ID, accountdomain.AccountStatus) error {
	return nil
}
func (s *accountRepoStub) UpdateProfile(context.Context, meta.ID, map[string]string) error {
	return nil
}
func (s *accountRepoStub) UpdateMeta(context.Context, meta.ID, map[string]string) error { return nil }
func (s *accountRepoStub) GetByID(context.Context, meta.ID) (*accountdomain.Account, error) {
	return nil, perrors.WithCode(code.ErrNotFoundAccount, "account not found")
}
func (s *accountRepoStub) GetByUniqueID(_ context.Context, uniqueID accountdomain.UnionID) (*accountdomain.Account, error) {
	if account, ok := s.byUniqueID[string(uniqueID)]; ok {
		return account, nil
	}
	return nil, perrors.WithCode(code.ErrNotFoundAccount, "account not found")
}
func (s *accountRepoStub) GetByExternalIDAppId(_ context.Context, externalID accountdomain.ExternalID, appID accountdomain.AppId) (*accountdomain.Account, error) {
	if account, ok := s.byExternalIDApp[string(externalID)+"|"+string(appID)]; ok {
		return account, nil
	}
	return nil, perrors.WithCode(code.ErrNotFoundAccount, "account not found")
}

func TestCreateOrGetUser_RepairsDanglingWechatAccountUser(t *testing.T) {
	t.Parallel()

	provisioner := newUserProvisioner(nil, nil)
	userRepo := &userRepoStub{users: make(map[uint64]*userdomain.User)}
	accountUserID := meta.FromUint64(615206334492586542)
	accountRepo := &accountRepoStub{
		byUniqueID: map[string]*accountdomain.Account{
			"union-1": accountdomain.NewAccount(accountUserID, accountdomain.TypeWcMinip, accountdomain.ExternalID("openid@app"), accountdomain.WithID(meta.FromUint64(1))),
		},
	}
	email, err := meta.NewEmail("clack@fangcunmount.com")
	require.NoError(t, err)

	req := OnboardingRequest{
		Name:           "clack",
		Email:          email,
		AccountType:    accountdomain.TypeWcMinip,
		CredentialType: CredTypeWechat,
		Profile: map[string]string{
			"nickname": "clack",
		},
	}

	user, isNew, err := provisioner.createOrGetUser(context.Background(), userRepo, accountRepo, req, "", "union-1")
	require.NoError(t, err)
	require.False(t, isNew)
	require.NotNil(t, user)
	require.Equal(t, accountUserID.Uint64(), user.ID.Uint64())
	require.Equal(t, "clack", user.Name)
	require.Equal(t, "clack", user.Nickname)
	require.Equal(t, "clack@fangcunmount.com", user.Email.String())

	stored, err := userRepo.FindByID(context.Background(), accountUserID)
	require.NoError(t, err)
	require.Equal(t, accountUserID.Uint64(), stored.ID.Uint64())
}

func TestPrepareWechatIdentityDoesNotMutateOriginalRequest(t *testing.T) {
	t.Parallel()

	appID := "wx-app"
	jsCode := "js-code"
	req := OnboardingRequest{
		AccountType:  accountdomain.TypeWcMinip,
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
			app: &idpwechatapp.WechatApp{
				AppID:  appID,
				Status: idpwechatapp.StatusEnabled,
				Cred: &idpwechatapp.Credentials{
					Auth: &idpwechatapp.AuthSecret{AppSecretCipher: []byte("cipher")},
				},
			},
		},
		onboardingSecretVaultStub{plaintext: "app-secret"},
	)

	identity, err := resolver.ResolveMiniProgram(context.Background(), OnboardingRequest{
		AccountType:  accountdomain.TypeWcMinip,
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

type onboardingWechatAppRepoStub struct {
	app *idpwechatapp.WechatApp
	err error
}

func (s *onboardingWechatAppRepoStub) Create(context.Context, *idpwechatapp.WechatApp) error {
	return nil
}

func (s *onboardingWechatAppRepoStub) GetByID(context.Context, idutil.ID) (*idpwechatapp.WechatApp, error) {
	return nil, nil
}

func (s *onboardingWechatAppRepoStub) GetByAppID(context.Context, string) (*idpwechatapp.WechatApp, error) {
	return s.app, s.err
}

func (s *onboardingWechatAppRepoStub) List(context.Context, idpwechatapp.ListFilter) ([]*idpwechatapp.WechatApp, error) {
	return nil, nil
}

func (s *onboardingWechatAppRepoStub) Update(context.Context, *idpwechatapp.WechatApp) error {
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
