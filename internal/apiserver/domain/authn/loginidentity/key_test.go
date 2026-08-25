package loginidentity

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestProviderSpecificKeyConstructorsAssignRealmSemantics(t *testing.T) {
	t.Parallel()

	tenantID := meta.FromUint64(9001)
	phone, err := meta.NewPhone("13811112222")
	require.NoError(t, err)

	tests := []struct {
		name       string
		construct  func() (ProviderKey, error)
		provider   Provider
		realm      string
		identifier string
		globalID   string
	}{
		{
			name:       "tenant username",
			construct:  func() (ProviderKey, error) { return NewUsernameProviderKey(tenantID, "zhangsan") },
			provider:   ProviderUsername,
			realm:      tenantID.String(),
			identifier: "zhangsan",
		},
		{
			name:       "default username",
			construct:  func() (ProviderKey, error) { return NewMockConsumerProviderKey("mock@example.com") },
			provider:   ProviderUsername,
			realm:      RealmDefault,
			identifier: "mock@example.com",
		},
		{
			name:       "global phone",
			construct:  func() (ProviderKey, error) { return NewPhoneProviderKey(phone) },
			provider:   ProviderPhone,
			realm:      RealmGlobal,
			identifier: "+8613811112222",
		},
		{
			name:       "wechat mini app",
			construct:  func() (ProviderKey, error) { return NewWechatMinipProviderKey("mini-app", "open-1", "union-1") },
			provider:   ProviderWechatMinip,
			realm:      "mini-app",
			identifier: "open-1",
			globalID:   "union-1",
		},
		{
			name:       "wechat open app",
			construct:  func() (ProviderKey, error) { return NewWechatOpenProviderKey("open-app", "open-2", "union-2") },
			provider:   ProviderWechatOpen,
			realm:      "open-app",
			identifier: "open-2",
			globalID:   "union-2",
		},
		{
			name:       "wecom corporation",
			construct:  func() (ProviderKey, error) { return NewWecomProviderKey("corp-1", "user-1") },
			provider:   ProviderWecom,
			realm:      "corp-1",
			identifier: "user-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			key, err := tt.construct()

			require.NoError(t, err)
			require.True(t, key.IsValid())
			require.Equal(t, tt.provider, key.Provider())
			require.Equal(t, tt.realm, key.Realm())
			require.Equal(t, tt.identifier, key.Identifier())
			require.Equal(t, tt.globalID, key.GlobalIdentifier())
		})
	}
}

func TestProviderSpecificKeyConstructorsRejectIncompleteInput(t *testing.T) {
	t.Parallel()

	phone := meta.Phone{}
	tests := []struct {
		name      string
		construct func() (ProviderKey, error)
	}{
		{name: "username identifier", construct: func() (ProviderKey, error) { return NewUsernameProviderKey(meta.ZeroID, " ") }},
		{name: "mock identifier", construct: func() (ProviderKey, error) { return NewMockConsumerProviderKey(" ") }},
		{name: "phone identifier", construct: func() (ProviderKey, error) { return NewPhoneProviderKey(phone) }},
		{name: "wechat mini realm", construct: func() (ProviderKey, error) { return NewWechatMinipProviderKey(" ", "open-1", "") }},
		{name: "wechat mini identifier", construct: func() (ProviderKey, error) { return NewWechatMinipProviderKey("mini-app", " ", "") }},
		{name: "wechat open realm", construct: func() (ProviderKey, error) { return NewWechatOpenProviderKey(" ", "open-1", "") }},
		{name: "wecom realm", construct: func() (ProviderKey, error) { return NewWecomProviderKey(" ", "user-1") }},
		{name: "wecom identifier", construct: func() (ProviderKey, error) { return NewWecomProviderKey("corp-1", " ") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			key, err := tt.construct()

			require.Error(t, err)
			require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
			require.False(t, key.IsValid())
		})
	}
}
