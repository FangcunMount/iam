package profile

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

func TestNewProfile_Success(t *testing.T) {
	id := meta.FromUint64(42)
	height, err := meta.NewHeightFromFloat(120.5)
	require.NoError(t, err)
	weight, err := meta.NewWeightFromFloat(35.2)
	require.NoError(t, err)
	birthday := meta.NewBirthday("2020-01-02")
	idCard, err := meta.NewIDCard("tester", "110101199003070011")
	require.NoError(t, err)

	profile, err := NewProfile(
		"小明",
		WithProfileID(id),
		WithGender(meta.GenderMale),
		WithBirthday(birthday),
		WithIDCard(idCard),
		WithHeight(height),
		WithWeight(weight),
	)

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, id, profile.ID)
	assert.Equal(t, "小明", profile.Name)
	assert.Equal(t, meta.GenderMale, profile.Gender)
	assert.True(t, profile.Birthday.Equal(birthday))
	assert.True(t, profile.IDCard.Equal(idCard))
	assert.Equal(t, height.Tenths(), profile.Height.Tenths())
	assert.Equal(t, weight.Tenths(), profile.Weight.Tenths())
}

func TestNewProfile_EmptyName(t *testing.T) {
	profile, err := NewProfile("")

	require.Error(t, err)
	assert.Nil(t, profile)
	assert.Contains(t, fmt.Sprintf("%-v", err), "name cannot be empty")
}

func TestNewFromCreationSpecUsesProfileCreationOptions(t *testing.T) {
	id := meta.FromUint64(99)
	idCard, err := meta.NewIDCard("tester", "110101199003070011")
	require.NoError(t, err)
	height, err := meta.NewHeightFromFloat(121.5)
	require.NoError(t, err)
	weight, err := meta.NewWeightFromFloat(33.4)
	require.NoError(t, err)
	birthday := meta.NewBirthday("2020-05-06")

	profile, err := NewFromCreationSpec(CreationSpec{
		ID:       id,
		Name:     "小新",
		IDCard:   idCard,
		Gender:   meta.GenderMale,
		Birthday: birthday,
		Height:   height,
		Weight:   weight,
	})

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, id, profile.ID)
	assert.Equal(t, "小新", profile.Name)
	assert.True(t, profile.IDCard.Equal(idCard))
	assert.Equal(t, meta.GenderMale, profile.Gender)
	assert.True(t, profile.Birthday.Equal(birthday))
	assert.Equal(t, height.Tenths(), profile.Height.Tenths())
	assert.Equal(t, weight.Tenths(), profile.Weight.Tenths())
}

func TestProfileRenamingAndProfileUpdates(t *testing.T) {
	profile, err := NewProfile("原名")
	require.NoError(t, err)

	newIDCard, err := meta.NewIDCard("tester", "110101199003070011")
	require.NoError(t, err)
	newBirthday := meta.NewBirthday("2019-02-03")
	newHeight, err := meta.NewHeightFromFloat(130.0)
	require.NoError(t, err)
	newWeight, err := meta.NewWeightFromFloat(40.0)
	require.NoError(t, err)

	profile.Rename("新名字")
	profile.UpdateIDCard(newIDCard)
	profile.UpdateProfile(meta.GenderFemale, newBirthday)
	profile.UpdateHeightWeight(newHeight, newWeight)

	assert.Equal(t, "新名字", profile.Name)
	assert.True(t, profile.IDCard.Equal(newIDCard))
	assert.Equal(t, meta.GenderFemale, profile.Gender)
	assert.True(t, profile.Birthday.Equal(newBirthday))
	assert.Equal(t, newHeight.Tenths(), profile.Height.Tenths())
	assert.Equal(t, newWeight.Tenths(), profile.Weight.Tenths())
}
