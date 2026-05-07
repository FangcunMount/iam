package user

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

func TestNewUser_Success(t *testing.T) {
	phone, _ := meta.NewPhone("+8613012345678")
	email, _ := meta.NewEmail("test@example.com")

	user, err := NewUser(
		"小明",
		phone,
		WithID(meta.FromUint64(10)),
		WithEmail(email),
		WithStatus(UserInactive),
	)

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "小明", user.Name)
	expectedPhone, _ := meta.NewPhone("+8613012345678")
	assert.True(t, user.Phone.Equal(expectedPhone))
	expectedEmail, _ := meta.NewEmail("test@example.com")
	assert.Equal(t, expectedEmail, user.Email)
	assert.Equal(t, UserInactive, user.Status)
}

func TestNewUser_Validations(t *testing.T) {
	validPhone, _ := meta.NewPhone("+8613012345678")
	_, err := NewUser("", validPhone)
	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "name cannot be empty")
	_, err = NewUser("name", meta.Phone{})
	require.NoError(t, err)
}

func TestUserLifecycleAndUpdates(t *testing.T) {
	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)
	user, err := NewUser("tester", phone)
	require.NoError(t, err)

	user.Deactivate()
	assert.True(t, user.IsInactive())

	user.Block()
	assert.True(t, user.IsBlocked())

	user.Activate()
	assert.True(t, user.IsUsable())

	require.NoError(t, user.Rename("new name"))
	assert.Equal(t, "new name", user.Name)

	newPhone, err := meta.NewPhone("+8613112345678")
	require.NoError(t, err)
	user.ChangePhone(newPhone)
	assert.True(t, user.Phone.Equal(newPhone))

	newEmail, err := meta.NewEmail("user@example.com")
	require.NoError(t, err)
	user.ChangeEmail(newEmail)
	assert.Equal(t, newEmail, user.Email)
}
