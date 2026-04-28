package account_test

import (
	"testing"

	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestNewAccount_DefaultsAndOptions(t *testing.T) {
	uid := meta.ID(123)
	acc := account.NewAccount(uid, account.TypeWcMinip, account.ExternalID("ext-1"))

	require.Equal(t, uid, acc.UserID)
	require.Equal(t, account.TypeWcMinip, acc.Type)
	require.Equal(t, account.ExternalID("ext-1"), acc.ExternalID)
	require.True(t, acc.IsActive())

	// option WithID
	acc2 := account.NewAccount(uid, account.TypeWcMinip, account.ExternalID("e"), account.WithID(meta.ID(999)), account.WithProfile(map[string]string{"nick": "tom"}))
	require.Equal(t, meta.ID(999), acc2.ID)
	v, ok := acc2.GetProfileField("nick")
	require.True(t, ok)
	require.Equal(t, "tom", v)
}

func TestSetUniqueID_Behavior(t *testing.T) {
	acc := account.NewAccount(0, account.TypeWcMinip, account.ExternalID("e"))
	ok := acc.SetUniqueID(account.UnionID("u1"))
	require.True(t, ok)
	require.Equal(t, account.UnionID("u1"), acc.UniqueID)

	// cannot set twice
	ok2 := acc.SetUniqueID(account.UnionID("u2"))
	require.False(t, ok2)
	require.Equal(t, account.UnionID("u1"), acc.UniqueID)
}

func TestProfileAndMeta_MergeAndFields(t *testing.T) {
	acc := account.NewAccount(0, account.TypeWcMinip, account.ExternalID("e"))
	acc.UpdateProfile(map[string]string{"a": "1", "b": "2"})
	acc.UpdateProfile(map[string]string{"b": "B", "c": "3"})
	require.Equal(t, "1", acc.Profile["a"])
	require.Equal(t, "B", acc.Profile["b"])
	require.Equal(t, "3", acc.Profile["c"])

	acc.SetProfileField("d", "4")
	v, ok := acc.GetProfileField("d")
	require.True(t, ok)
	require.Equal(t, "4", v)

	acc.UpdateMeta(map[string]string{"x": "1"})
	acc.SetMetaField("y", "2")
	mv, mok := acc.GetMetaField("x")
	require.True(t, mok)
	require.Equal(t, "1", mv)
}

func TestCanTransitionTo_Rules(t *testing.T) {
	acc := account.NewAccount(0, account.TypeWcMinip, account.ExternalID("e"))
	// Active -> Disabled
	require.True(t, acc.CanTransitionTo(account.StatusDisabled))
	// Active -> Active (idempotent)
	require.True(t, acc.CanTransitionTo(account.StatusActive))

	acc.Disable()
	// Disabled -> Active
	require.True(t, acc.CanTransitionTo(account.StatusActive))

	acc.Archive()
	// Archived -> Active
	require.True(t, acc.CanTransitionTo(account.StatusActive))

	acc.Delete()
	// Deleted -> only Deleted
	require.False(t, acc.CanTransitionTo(account.StatusActive))
	require.True(t, acc.CanTransitionTo(account.StatusDeleted))
}

func TestCanUpdate_WhenDeleted(t *testing.T) {
	acc := account.NewAccount(0, account.TypeWcMinip, account.ExternalID("e"))
	require.True(t, acc.CanUpdateProfile())
	require.True(t, acc.CanUpdateMeta())

	acc.Delete()
	require.False(t, acc.CanUpdateProfile())
	require.False(t, acc.CanUpdateMeta())
}
