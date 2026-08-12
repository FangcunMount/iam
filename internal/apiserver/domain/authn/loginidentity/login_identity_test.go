package loginidentity

import (
	"testing"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestLoginIdentityBuilderBuildsProviderKeys(t *testing.T) {
	userID := meta.FromUint64(1001)

	username, err := NewBuilder(userID).Username("tenant-A", "zhangsan").Build()
	require.NoError(t, err)
	require.Equal(t, ProviderUsername, username.Provider)
	require.Equal(t, "tenant-A", username.Realm)
	require.Equal(t, "zhangsan", username.Identifier)
	require.True(t, username.IsActive())

	phone, err := NewBuilder(userID).Phone("+8613811112222").Build()
	require.NoError(t, err)
	require.Equal(t, ProviderPhone, phone.Provider)
	require.Equal(t, RealmGlobal, phone.Realm)
	require.Equal(t, "+8613811112222", phone.Identifier)

	wechat, err := NewBuilder(userID).WechatMinip("wx-app", "openid-1", "union-1").Build()
	require.NoError(t, err)
	require.Equal(t, ProviderWechatMinip, wechat.Provider)
	require.Equal(t, "wx-app", wechat.Realm)
	require.Equal(t, "openid-1", wechat.Identifier)
	require.Equal(t, "union-1", wechat.GlobalIdentifier)

	// 测试微信开放平台登录身份
	wechatOpen, err := NewBuilder(userID).WechatOpen("wx-app", "openid-1", "union-1").Build()
	require.NoError(t, err)
	require.Equal(t, ProviderWechatOpen, wechatOpen.Provider)
	require.Equal(t, "wx-app", wechatOpen.Realm)
	require.Equal(t, "openid-1", wechatOpen.Identifier)
	require.Equal(t, "union-1", wechatOpen.GlobalIdentifier)

	wecom, err := NewBuilder(userID).Wecom("corp-1", "userid-1").Build()
	require.NoError(t, err)
	require.Equal(t, ProviderWecom, wecom.Provider)
	require.Equal(t, "corp-1", wecom.Realm)
	require.Equal(t, "userid-1", wecom.Identifier)
}

func TestLoginIdentityBuilderRejectsIncompleteProviderKey(t *testing.T) {
	userID := meta.FromUint64(1001)

	_, err := NewBuilder(meta.ZeroID).Username("tenant-A", "zhangsan").Build()
	require.Error(t, err)

	_, err = NewBuilder(userID).Username("", "zhangsan").Build()
	require.NoError(t, err)

	_, err = NewBuilder(userID).WechatMinip("", "openid", "").Build()
	require.Error(t, err)

	_, err = NewBuilder(userID).WechatMinip("wx-app", "", "").Build()
	require.Error(t, err)
}
