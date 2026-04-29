package profile_test

import (
	"context"
	"errors"
	"testing"

	profile "github.com/FangcunMount/iam/internal/apiserver/domain/uc/profile"
	"github.com/FangcunMount/iam/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileEditor_RenameSuccess(t *testing.T) {
	id := meta.FromUint64(1)
	ch := &profile.Profile{ID: id, Name: "Old"}
	repo := &testhelpers.ProfileRepoStub{Profile: ch}
	editor := profile.NewProfileService(repo, &testhelpers.ProfileValidatorStub{})

	updated, err := editor.Rename(context.Background(), ch.ID, "NewName")

	require.NoError(t, err)
	assert.Equal(t, "NewName", ch.Name)
	assert.Same(t, ch, updated)
	assert.Equal(t, 1, repo.FindCalls)
}

func TestProfileEditor_RenameValidatorError(t *testing.T) {
	id := meta.FromUint64(1)
	repo := &testhelpers.ProfileRepoStub{Profile: &profile.Profile{ID: id, Name: "Old"}}
	editor := profile.NewProfileService(repo, &testhelpers.ProfileValidatorStub{RenameErr: errors.New("invalid name")})

	updated, err := editor.Rename(context.Background(), repo.Profile.ID, "bad")

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.Equal(t, 0, repo.FindCalls, "repository should not be called when validation fails")
}

func TestProfileEditor_RenameRepoError(t *testing.T) {
	repo := &testhelpers.ProfileRepoStub{Profile: &profile.Profile{ID: meta.FromUint64(1)}, FindErr: errors.New("db error")}
	editor := profile.NewProfileService(repo, &testhelpers.ProfileValidatorStub{})

	updated, err := editor.Rename(context.Background(), repo.Profile.ID, "Name")

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.Equal(t, 1, repo.FindCalls)
}

func TestProfileEditor_UpdateProfileSuccess(t *testing.T) {
	ch := &profile.Profile{ID: meta.FromUint64(2)}
	repo := &testhelpers.ProfileRepoStub{Profile: ch}
	editor := profile.NewProfileService(repo, &testhelpers.ProfileValidatorStub{})

	birthday := meta.NewBirthday("2020-05-06")
	updated, err := editor.UpdateProfile(context.Background(), ch.ID, meta.GenderFemale, birthday)

	require.NoError(t, err)
	assert.Same(t, ch, updated)
	assert.Equal(t, meta.GenderFemale, ch.Gender)
	assert.True(t, ch.Birthday.Equal(birthday))
}

func TestProfileEditor_UpdateProfileValidatorError(t *testing.T) {
	repo := &testhelpers.ProfileRepoStub{Profile: &profile.Profile{ID: meta.FromUint64(3)}}
	editor := profile.NewProfileService(repo, &testhelpers.ProfileValidatorStub{UpdateProfileErr: errors.New("bad birthday")})

	updated, err := editor.UpdateProfile(context.Background(), repo.Profile.ID, meta.GenderMale, meta.Birthday{})

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.Equal(t, 0, repo.FindCalls)
}

func TestProfileEditor_UpdateHeightWeight(t *testing.T) {
	ch := &profile.Profile{ID: meta.FromUint64(4)}
	repo := &testhelpers.ProfileRepoStub{Profile: ch}
	editor := profile.NewProfileService(repo, &testhelpers.ProfileValidatorStub{})

	height, err := meta.NewHeightFromFloat(150.4)
	require.NoError(t, err)
	weight, err := meta.NewWeightFromFloat(45.1)
	require.NoError(t, err)

	updated, err := editor.UpdateHeightWeight(context.Background(), ch.ID, height, weight)

	require.NoError(t, err)
	assert.Same(t, ch, updated)
	assert.Equal(t, height.Tenths(), ch.Height.Tenths())
	assert.Equal(t, weight.Tenths(), ch.Weight.Tenths())
}

func TestProfileEditor_UpdateIDCard(t *testing.T) {
	ch := &profile.Profile{ID: meta.FromUint64(5)}
	repo := &testhelpers.ProfileRepoStub{Profile: ch}
	editor := profile.NewProfileService(repo, &testhelpers.ProfileValidatorStub{})

	idCard, err := meta.NewIDCard("tester", "110101199003070011")
	require.NoError(t, err)
	updated, err := editor.UpdateIDCard(context.Background(), ch.ID, idCard)

	require.NoError(t, err)
	assert.Same(t, ch, updated)
	assert.True(t, ch.IDCard.Equal(idCard))
}
