package loginidentity

import (
	"testing"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestLoginIdentityBuilderBuildsProviderKeys(t *testing.T) {
	userID := meta.FromUint64(1001)
	tenantID := meta.FromUint64(9001)

	usernameKey, err := NewUsernameProviderKey(tenantID, "zhangsan")
	require.NoError(t, err)
	username, err := NewBuilder(userID).FromProviderKey(usernameKey).Build()
	require.NoError(t, err)
	require.Equal(t, ProviderUsername, username.Provider)
	require.Equal(t, tenantID.String(), username.Realm)
	require.Equal(t, "zhangsan", username.Identifier)
	require.True(t, username.IsActive())

	phoneNumber, err := meta.NewPhone("13811112222")
	require.NoError(t, err)
	phoneKey, err := NewPhoneProviderKey(phoneNumber)
	require.NoError(t, err)
	phone, err := NewBuilder(userID).FromProviderKey(phoneKey).Build()
	require.NoError(t, err)
	require.Equal(t, ProviderPhone, phone.Provider)
	require.Equal(t, RealmGlobal, phone.Realm)
	require.Equal(t, "+8613811112222", phone.Identifier)

	wechatKey, err := NewWechatMinipProviderKey("wx-app", "openid-1", "union-1")
	require.NoError(t, err)
	wechat, err := NewBuilder(userID).FromProviderKey(wechatKey).Build()
	require.NoError(t, err)
	require.Equal(t, ProviderWechatMinip, wechat.Provider)
	require.Equal(t, "wx-app", wechat.Realm)
	require.Equal(t, "openid-1", wechat.Identifier)
	require.Equal(t, "union-1", wechat.GlobalIdentifier)

	// 测试微信开放平台登录身份
	wechatOpenKey, err := NewWechatOpenProviderKey("wx-app", "openid-1", "union-1")
	require.NoError(t, err)
	wechatOpen, err := NewBuilder(userID).FromProviderKey(wechatOpenKey).Build()
	require.NoError(t, err)
	require.Equal(t, ProviderWechatOpen, wechatOpen.Provider)
	require.Equal(t, "wx-app", wechatOpen.Realm)
	require.Equal(t, "openid-1", wechatOpen.Identifier)
	require.Equal(t, "union-1", wechatOpen.GlobalIdentifier)

	wecomKey, err := NewWecomProviderKey("corp-1", "userid-1")
	require.NoError(t, err)
	wecom, err := NewBuilder(userID).FromProviderKey(wecomKey).Build()
	require.NoError(t, err)
	require.Equal(t, ProviderWecom, wecom.Provider)
	require.Equal(t, "corp-1", wecom.Realm)
	require.Equal(t, "userid-1", wecom.Identifier)
}

func TestLoginIdentityBuilderRejectsZeroUserID(t *testing.T) {
	key, err := NewUsernameProviderKey(meta.FromUint64(9001), "zhangsan")
	require.NoError(t, err)

	_, err = NewBuilder(meta.ZeroID).FromProviderKey(key).Build()
	require.Error(t, err)
}
