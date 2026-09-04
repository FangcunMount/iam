package authnclaims

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeSnapshotUsesJSONForNonStringValues(t *testing.T) {
	got := EncodeSnapshot(map[string]any{"n": 42})
	require.Equal(t, `42`, got["n"])
}

func TestEncodeJWTAttributesUsesAllowlist(t *testing.T) {
	got := EncodeJWTAttributes(map[string]any{
		"user_id":      "1",
		"auth_time":    "2026-01-02T03:04:05Z",
		"phone_number": "+8613800138000",
		"wx_openid":    "openid",
		"custom_key":   "ok",
	})
	require.Equal(t, map[string]string{"auth_time": "2026-01-02T03:04:05Z"}, got)
}

func TestEncodeServiceAttributesUsesAllowlist(t *testing.T) {
	got := EncodeServiceAttributes(map[string]string{
		"scope": "internal",
		"level": "2",
		"secret": "nope",
	})
	require.Equal(t, map[string]string{"scope": "internal", "level": "2"}, got)
}
