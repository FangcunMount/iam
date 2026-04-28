package child

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

func TestNewChild_Success(t *testing.T) {
	id := meta.FromUint64(42)
	height, err := meta.NewHeightFromFloat(120.5)
	require.NoError(t, err)
	weight, err := meta.NewWeightFromFloat(35.2)
	require.NoError(t, err)
	birthday := meta.NewBirthday("2020-01-02")
	idCard, err := meta.NewIDCard("tester", "110101199003070011")
	require.NoError(t, err)

	child, err := NewChild(
		"小明",
		WithChildID(id),
		WithGender(meta.GenderMale),
		WithBirthday(birthday),
		WithIDCard(idCard),
		WithHeight(height),
		WithWeight(weight),
	)

	require.NoError(t, err)
	require.NotNil(t, child)
	assert.Equal(t, id, child.ID)
	assert.Equal(t, "小明", child.Name)
	assert.Equal(t, meta.GenderMale, child.Gender)
	assert.True(t, child.Birthday.Equal(birthday))
	assert.True(t, child.IDCard.Equal(idCard))
	assert.Equal(t, height.Tenths(), child.Height.Tenths())
	assert.Equal(t, weight.Tenths(), child.Weight.Tenths())
}

func TestNewChild_EmptyName(t *testing.T) {
	child, err := NewChild("")

	require.Error(t, err)
	assert.Nil(t, child)
	assert.Contains(t, fmt.Sprintf("%-v", err), "name cannot be empty")
}

func TestChildRenamingAndProfileUpdates(t *testing.T) {
	child, err := NewChild("原名")
	require.NoError(t, err)

	newIDCard, err := meta.NewIDCard("tester", "110101199003070011")
	require.NoError(t, err)
	newBirthday := meta.NewBirthday("2019-02-03")
	newHeight, err := meta.NewHeightFromFloat(130.0)
	require.NoError(t, err)
	newWeight, err := meta.NewWeightFromFloat(40.0)
	require.NoError(t, err)

	child.Rename("新名字")
	child.UpdateIDCard(newIDCard)
	child.UpdateProfile(meta.GenderFemale, newBirthday)
	child.UpdateHeightWeight(newHeight, newWeight)

	assert.Equal(t, "新名字", child.Name)
	assert.True(t, child.IDCard.Equal(newIDCard))
	assert.Equal(t, meta.GenderFemale, child.Gender)
	assert.True(t, child.Birthday.Equal(newBirthday))
	assert.Equal(t, newHeight.Tenths(), child.Height.Tenths())
	assert.Equal(t, newWeight.Tenths(), child.Weight.Tenths())
}
