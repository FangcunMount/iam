package profile

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

func TestNewProfile_Success(t *testing.T) {
	id := meta.FromUint64(42)
	birthday := meta.NewBirthday("2020-01-02")
	idCard, err := meta.NewIDCard("tester", "110101199003070011")
	require.NoError(t, err)

	profile, err := NewProfile(
		"小明",
		WithGender(meta.GenderMale),
		WithBirthday(birthday),
		WithIDCard(idCard),
	)

	require.NoError(t, err)
	require.NotNil(t, profile)
	profile.ID = id
	assert.Equal(t, id, profile.ID)
	assert.Equal(t, "小明", profile.Name)
	assert.Equal(t, meta.GenderMale, profile.Gender)
	assert.True(t, profile.Birthday.Equal(birthday))
	assert.True(t, profile.IDCard.Equal(idCard))
}

func TestNewProfile_EmptyName(t *testing.T) {
	profile, err := NewProfile("")

	require.Error(t, err)
	assert.Nil(t, profile)
	assert.Contains(t, fmt.Sprintf("%-v", err), "name cannot be empty")
}

func TestProfileRenamingAndProfileUpdates(t *testing.T) {
	profile, err := NewProfile("原名")
	require.NoError(t, err)

	newIDCard, err := meta.NewIDCard("tester", "110101199003070011")
	require.NoError(t, err)
	newBirthday := meta.NewBirthday("2019-02-03")

	require.NoError(t, profile.Rename("新名字"))
	profile.UpdateIDCard(newIDCard)
	profile.UpdateProfile(meta.GenderFemale, newBirthday)

	assert.Equal(t, "新名字", profile.Name)
	assert.True(t, profile.IDCard.Equal(newIDCard))
	assert.Equal(t, meta.GenderFemale, profile.Gender)
	assert.True(t, profile.Birthday.Equal(newBirthday))
}
