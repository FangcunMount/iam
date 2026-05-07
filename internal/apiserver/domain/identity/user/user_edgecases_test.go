package user

import (
	"testing"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdatePhoneEmailEdgecases(t *testing.T) {
	phone1, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)
	u, err := NewUser("name", phone1)
	require.NoError(t, err)

	// Update phone to another valid phone
	p2, err := meta.NewPhone("+8613112345678")
	require.NoError(t, err)
	u.ChangePhone(p2)
	assert.True(t, u.Phone.Equal(p2))

	// Update email to valid one
	e, err := meta.NewEmail("hello@example.com")
	require.NoError(t, err)
	u.ChangeEmail(e)
	assert.Equal(t, e, u.Email)
}
