package account

import (
	"testing"

	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAccount_And_Options(t *testing.T) {
	userID := meta.FromUint64(10)
	acc := NewAccount(userID, TypeWcMinip, ExternalID("ext-1"))

	require.NotNil(t, acc)
	// NewAccount 首参应当作为关联 UserID
	assert.Equal(t, userID, acc.UserID)
	// 默认情况下 ID 由持久层生成，应为零值
	assert.Equal(t, uint64(0), acc.ID.Uint64())
	assert.Equal(t, TypeWcMinip, acc.Type)
	assert.Equal(t, ExternalID("ext-1"), acc.ExternalID)
	assert.True(t, acc.IsActive())

	// WithID 与 WithAppID 生效
	id := meta.FromUint64(123)
	acc2 := NewAccount(userID, TypeWcMinip, ExternalID("x"), WithID(id), WithAppID(AppId("app-x")))
	require.NotNil(t, acc2)
	assert.Equal(t, id, acc2.ID)
	assert.Equal(t, AppId("app-x"), acc2.AppID)
}

func TestSetUniqueID_Idempotency(t *testing.T) {
	userID := meta.FromUint64(11)
	acc := NewAccount(userID, TypeWcMinip, ExternalID("e"))

	ok := acc.SetUniqueID(UnionID("u-1"))
	assert.True(t, ok)
	assert.Equal(t, UnionID("u-1"), acc.UniqueID)

	// 再次设置应该被拒绝
	ok2 := acc.SetUniqueID(UnionID("u-2"))
	assert.False(t, ok2)
	assert.Equal(t, UnionID("u-1"), acc.UniqueID)
}

func TestProfileAndMetaMerge(t *testing.T) {
	userID := meta.FromUint64(12)
	acc := NewAccount(userID, TypeWcMinip, ExternalID("e2"))

	acc.UpdateProfile(map[string]string{"name": "A", "age": "5"})
	assert.Equal(t, "A", acc.Profile["name"])
	assert.Equal(t, "5", acc.Profile["age"])

	// merge 更新
	acc.UpdateProfile(map[string]string{"age": "6", "city": "X"})
	assert.Equal(t, "6", acc.Profile["age"])
	assert.Equal(t, "X", acc.Profile["city"])

	// meta 同理
	acc.UpdateMeta(map[string]string{"k1": "v1"})
	assert.Equal(t, "v1", acc.Meta["k1"])
}

func TestStatusTransitions(t *testing.T) {
	userID := meta.FromUint64(13)
	acc := NewAccount(userID, TypeWcMinip, ExternalID("e3"))

	// 默认 Active
	assert.True(t, acc.CanTransitionTo(StatusDisabled))
	assert.True(t, acc.CanTransitionTo(StatusArchived))
	assert.False(t, acc.CanTransitionTo(-99))

	// Disabled -> Active
	acc.Disable()
	assert.True(t, acc.IsDisabled())
	assert.True(t, acc.CanTransitionTo(StatusActive))
}

func TestIsSameUserAndHelpers(t *testing.T) {
	acc := NewAccount(meta.FromUint64(6), AccountType("t"), ExternalID("e"))
	assert.True(t, acc.CanUpdateProfile())
	assert.True(t, acc.CanUpdateMeta())

	acc.Delete()
	assert.False(t, acc.CanUpdateProfile())
	assert.False(t, acc.CanUpdateMeta())
}
