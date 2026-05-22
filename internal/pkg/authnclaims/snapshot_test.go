package authnclaims

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeSnapshotUsesJSONForNonStringValues(t *testing.T) {
	got := EncodeSnapshot(map[string]any{"n": 42})
	require.Equal(t, `42`, got["n"])
}

func TestEncodeJWTAttributesSkipsReservedKeys(t *testing.T) {
	got := EncodeJWTAttributes(map[string]any{
		"user_id":    "1",
		"custom_key": "ok",
	})
	require.NotContains(t, got, "user_id")
	require.Equal(t, "ok", got["custom_key"])
}
