package user

import (
	"testing"

	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUser_WithZeroIDCardAndUpdates(t *testing.T) {
	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)

	// Create user with zero IDCard (value type zero) via WithIDCard
	user, err := NewUser("u1", phone, WithID(meta.FromUint64(100)), WithIDCard(meta.IDCard{}))
	require.NoError(t, err)
	require.NotNil(t, user)

	// Zero IDCard should serialize to empty number
	assert.Equal(t, "", user.IDCard.Number())

	// UpdateIDCard with a valid card
	idc, err := meta.NewIDCard("name", "110101199003070011")
	require.NoError(t, err)
	user.UpdateIDCard(idc)
	assert.Equal(t, idc.Number(), user.IDCard.Number())
}

func TestUpdatePhoneEmailEdgecases(t *testing.T) {
	phone1, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)
	u, err := NewUser("name", phone1)
	require.NoError(t, err)

	// Update phone to another valid phone
	p2, err := meta.NewPhone("+8613112345678")
	require.NoError(t, err)
	u.UpdatePhone(p2)
	assert.True(t, u.Phone.Equal(p2))

	// Update email to valid one
	e, err := meta.NewEmail("hello@example.com")
	require.NoError(t, err)
	u.UpdateEmail(e)
	assert.Equal(t, e, u.Email)
}
