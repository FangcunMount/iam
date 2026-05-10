package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountProviderKeyMapsLegacyAccountTypes(t *testing.T) {
	tests := []struct {
		name string
		acc  legacyAccount
		want providerKey
		ok   bool
	}{
		{
			name: "opera username scoped by tenant",
			acc: legacyAccount{
				Type:           "opera",
				ExternalID:     "alice",
				ScopedTenantID: 42,
			},
			want: providerKey{Provider: providerUsername, Realm: "42", Identifier: "alice"},
			ok:   true,
		},
		{
			name: "mock consumer username default realm",
			acc:  legacyAccount{Type: "mock-consumer", ExternalID: "consumer-1"},
			want: providerKey{Provider: providerUsername, Realm: realmDefault, Identifier: "consumer-1"},
			ok:   true,
		},
		{
			name: "wechat minip",
			acc: legacyAccount{
				Type:       "wc-minip",
				AppID:      "wx-app",
				ExternalID: "openid",
				UniqueID:   "unionid",
			},
			want: providerKey{Provider: providerWechatMinip, Realm: "wx-app", Identifier: "openid", GlobalIdentifier: "unionid"},
			ok:   true,
		},
		{
			name: "wecom",
			acc:  legacyAccount{Type: "wc-com", AppID: "corp", ExternalID: "userid"},
			want: providerKey{Provider: providerWecom, Realm: "corp", Identifier: "userid"},
			ok:   true,
		},
		{
			name: "unsupported official account",
			acc:  legacyAccount{Type: "wc-offi", AppID: "wx-offi", ExternalID: "openid"},
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := accountProviderKey(tt.acc)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestStatusMapping(t *testing.T) {
	require.Equal(t, statusDisabled, accountStatus(0))
	require.Equal(t, statusActive, accountStatus(1))
	require.Equal(t, statusArchived, accountStatus(2))
	require.Equal(t, statusDeleted, accountStatus(3))
	require.Equal(t, statusDisabled, accountStatus(99))

	require.Equal(t, credentialStatusEnabled, credentialStatus(1))
	require.Equal(t, credentialStatusDisabled, credentialStatus(0))
}

func TestCredentialTypeDetection(t *testing.T) {
	require.True(t, isPasswordCredential(legacyCredential{Type: "password", Material: []byte("hash"), Algo: "argon2id"}))
	require.True(t, isPasswordCredential(legacyCredential{Material: []byte("hash"), Algo: "bcrypt"}))
	require.False(t, isPasswordCredential(legacyCredential{Type: "password"}))

	require.True(t, isPhoneCredential(legacyCredential{Type: "phone_otp"}))
	require.True(t, isPhoneCredential(legacyCredential{IDP: "phone"}))
	require.False(t, isPhoneCredential(legacyCredential{Type: "oauth_wx_minip", IDP: "wechat"}))
}

func TestMergeLegacyMetaPreservesObjectAndAddsLegacyFields(t *testing.T) {
	got := mergeLegacyMeta([]byte(`{"source":"old"}`), map[string]any{"legacy_account_id": uint64(12)})

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(got, &decoded))
	require.Equal(t, "old", decoded["source"])
	require.Equal(t, float64(12), decoded["legacy_account_id"])
}

func TestMergeLegacyMetaStoresInvalidJSONAsRaw(t *testing.T) {
	got := mergeLegacyMeta([]byte(`not-json`), map[string]any{"legacy_table": "auth_credentials"})

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(got, &decoded))
	require.Equal(t, "not-json", decoded["legacy_meta_raw"])
	require.Equal(t, "auth_credentials", decoded["legacy_table"])
}
