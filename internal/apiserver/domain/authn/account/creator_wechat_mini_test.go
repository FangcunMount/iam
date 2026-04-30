package account

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestWechatMinipCreatorRequiresPreparedOpenID(t *testing.T) {
	t.Parallel()

	appID := "wx-app"
	creator := NewAccountCreator(&accountCreatorRepoStub{})

	_, _, err := creator.CreateAccount(context.Background(), CreationInput{
		UserID:      meta.FromUint64(101),
		AccountType: TypeWcMinip,
		WechatAppID: &appID,
	})

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestWechatMinipCreatorUsesPreparedIdentity(t *testing.T) {
	t.Parallel()

	appID := "wx-app"
	openID := "openid-1"
	unionID := "union-1"
	creator := NewAccountCreator(&accountCreatorRepoStub{})

	account, params, err := creator.CreateAccount(context.Background(), CreationInput{
		UserID:        meta.FromUint64(101),
		AccountType:   TypeWcMinip,
		WechatAppID:   &appID,
		WechatOpenID:  &openID,
		WechatUnionID: &unionID,
		Profile:       map[string]string{"nickname": "alice"},
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.NotNil(t, params)
	require.Equal(t, TypeWcMinip, account.Type)
	require.Equal(t, ExternalID("openid-1@wx-app"), account.ExternalID)
	require.Equal(t, AppId("wx-app"), params.AppID)
	require.Equal(t, "openid-1", params.OpenID)
	require.Equal(t, "union-1", params.UnionID)
	require.Equal(t, "alice", account.Profile["nickname"])
}

type accountCreatorRepoStub struct{}

func (s *accountCreatorRepoStub) Create(context.Context, *Account) error {
	return nil
}

func (s *accountCreatorRepoStub) UpdateUniqueID(context.Context, meta.ID, UnionID) error {
	return nil
}

func (s *accountCreatorRepoStub) UpdateStatus(context.Context, meta.ID, AccountStatus) error {
	return nil
}

func (s *accountCreatorRepoStub) UpdateProfile(context.Context, meta.ID, map[string]string) error {
	return nil
}

func (s *accountCreatorRepoStub) UpdateMeta(context.Context, meta.ID, map[string]string) error {
	return nil
}

func (s *accountCreatorRepoStub) GetByID(context.Context, meta.ID) (*Account, error) {
	return nil, nil
}

func (s *accountCreatorRepoStub) GetByUniqueID(context.Context, UnionID) (*Account, error) {
	return nil, nil
}

func (s *accountCreatorRepoStub) GetByExternalIDAppId(context.Context, ExternalID, AppId) (*Account, error) {
	return nil, nil
}
