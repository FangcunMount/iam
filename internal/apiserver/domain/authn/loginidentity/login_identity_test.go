package loginidentity

import (
	"testing"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestLoginIdentityFactoriesBuildProviderKeys(t *testing.T) {
	userID := meta.FromUint64(1001)

	username, err := NewUsernameIdentity(userID, "tenant-A", "zhangsan")
	require.NoError(t, err)
	require.Equal(t, ProviderUsername, username.Provider)
	require.Equal(t, "tenant-A", username.Realm)
	require.Equal(t, "zhangsan", username.Identifier)
	require.True(t, username.IsActive())

	phone, err := NewPhoneIdentity(userID, "+8613811112222")
	require.NoError(t, err)
	require.Equal(t, ProviderPhone, phone.Provider)
	require.Equal(t, RealmGlobal, phone.Realm)
	require.Equal(t, "+8613811112222", phone.Identifier)

	wechat, err := NewWechatMinipIdentity(userID, "wx-app", "openid-1", "union-1")
	require.NoError(t, err)
	require.Equal(t, ProviderWechatMinip, wechat.Provider)
	require.Equal(t, "wx-app", wechat.Realm)
	require.Equal(t, "openid-1", wechat.Identifier)
	require.Equal(t, "union-1", wechat.GlobalIdentifier)

	wecom, err := NewWecomIdentity(userID, "corp-1", "userid-1")
	require.NoError(t, err)
	require.Equal(t, ProviderWecom, wecom.Provider)
	require.Equal(t, "corp-1", wecom.Realm)
	require.Equal(t, "userid-1", wecom.Identifier)
}

func TestLoginIdentityFactoryRejectsIncompleteProviderKey(t *testing.T) {
	userID := meta.FromUint64(1001)

	_, err := NewUsernameIdentity(meta.ZeroID, "tenant-A", "zhangsan")
	require.Error(t, err)

	_, err = NewUsernameIdentity(userID, "", "zhangsan")
	require.NoError(t, err)

	_, err = NewWechatMinipIdentity(userID, "", "openid", "")
	require.Error(t, err)

	_, err = NewWechatMinipIdentity(userID, "wx-app", "", "")
	require.Error(t, err)
}
