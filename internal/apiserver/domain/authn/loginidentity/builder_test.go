package loginidentity

import (
	"testing"
	"time"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestLoginIdentityBuilderBuildsFromProviderKey(t *testing.T) {
	t.Parallel()

	userID := meta.FromUint64(1001)
	verifiedAt := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	profile := map[string]string{"nickname": "alice"}
	metaData := map[string]string{"source": "test"}

	identity, err := NewBuilder(userID).
		FromProviderKey(WechatMinipProviderKey("wx-app", "openid-1", "union-1")).
		WithVerifiedAt(verifiedAt).
		WithProfile(profile).
		WithMeta(metaData).
		Build()

	require.NoError(t, err)
	require.Equal(t, userID, identity.UserID)
	require.Equal(t, ProviderWechatMinip, identity.Provider)
	require.Equal(t, "wx-app", identity.Realm)
	require.Equal(t, "openid-1", identity.Identifier)
	require.Equal(t, "union-1", identity.GlobalIdentifier)
	require.Equal(t, &verifiedAt, identity.VerifiedAt)
	require.Equal(t, profile, identity.Profile)
	require.Equal(t, metaData, identity.Meta)

	profile["nickname"] = "changed"
	metaData["source"] = "changed"
	require.Equal(t, "alice", identity.Profile["nickname"])
	require.Equal(t, "test", identity.Meta["source"])
}

func TestLoginIdentityBuilderSemanticMethods(t *testing.T) {
	t.Parallel()

	userID := meta.FromUint64(1001)

	username, err := NewBuilder(userID).Username("", "zhangsan").Build()
	require.NoError(t, err)
	require.Equal(t, ProviderUsername, username.Provider)
	require.Equal(t, RealmDefault, username.Realm)
	require.Equal(t, "zhangsan", username.Identifier)

	phone, err := NewBuilder(userID).Phone("+8613811112222").Build()
	require.NoError(t, err)
	require.Equal(t, ProviderPhone, phone.Provider)
	require.Equal(t, RealmGlobal, phone.Realm)
	require.Equal(t, "+8613811112222", phone.Identifier)

	wecom, err := NewBuilder(userID).Wecom("corp-1", "userid-1").Build()
	require.NoError(t, err)
	require.Equal(t, ProviderWecom, wecom.Provider)
	require.Equal(t, "corp-1", wecom.Realm)
	require.Equal(t, "userid-1", wecom.Identifier)
}

func TestLoginIdentityBuilderRejectsMissingProviderKey(t *testing.T) {
	t.Parallel()

	_, err := NewBuilder(meta.FromUint64(1001)).Build()
	require.Error(t, err)
}
