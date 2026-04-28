package register

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	accountdomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	userdomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
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

	resolver := newUserResolver(nil, nil, nil, nil)
	userRepo := &userRepoStub{users: make(map[uint64]*userdomain.User)}
	accountUserID := meta.FromUint64(615206334492586542)
	accountRepo := &accountRepoStub{
		byUniqueID: map[string]*accountdomain.Account{
			"union-1": accountdomain.NewAccount(accountUserID, accountdomain.TypeWcMinip, accountdomain.ExternalID("openid@app"), accountdomain.WithID(meta.FromUint64(1))),
		},
	}
	email, err := meta.NewEmail("clack@fangcunmount.com")
	require.NoError(t, err)

	req := RegisterRequest{
		Name:           "clack",
		Email:          email,
		AccountType:    accountdomain.TypeWcMinip,
		CredentialType: CredTypeWechat,
		Profile: map[string]string{
			"nickname": "clack",
		},
	}

	user, isNew, err := resolver.createOrGetUser(context.Background(), userRepo, accountRepo, req, "", "union-1")
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
