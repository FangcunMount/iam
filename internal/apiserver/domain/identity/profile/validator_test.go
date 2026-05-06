package profile_test

import (
	"context"
	"fmt"
	"testing"

	profile "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
	"github.com/FangcunMount/iam/v2/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileValidator_ValidateCreate(t *testing.T) {
	v := profile.NewValidator(&testhelpers.ProfileRepoStub{})
	err := v.ValidateCreate(context.Background(), "name", meta.GenderMale, meta.NewBirthday("2010-01-01"))
	assert.NoError(t, err)
}

func TestProfileValidator_ValidateCreate_EmptyName(t *testing.T) {
	v := profile.NewValidator(&testhelpers.ProfileRepoStub{})
	err := v.ValidateCreate(context.Background(), "", meta.GenderMale, meta.NewBirthday("2010-01-01"))
	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "name cannot be empty")
}

func TestProfileValidator_ValidateCreate_EmptyBirthdayAllowed(t *testing.T) {
	v := profile.NewValidator(&testhelpers.ProfileRepoStub{})
	err := v.ValidateCreate(context.Background(), "name", meta.GenderMale, meta.Birthday{})
	require.NoError(t, err)
}

func TestProfileValidator_ValidateRename(t *testing.T) {
	v := profile.NewValidator(&testhelpers.ProfileRepoStub{})
	assert.NoError(t, v.ValidateRename("valid"))

	err := v.ValidateRename("")
	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "name cannot be empty")
}

func TestProfileValidator_ValidateUpdateProfile(t *testing.T) {
	v := profile.NewValidator(&testhelpers.ProfileRepoStub{})
	assert.NoError(t, v.ValidateUpdateProfile(meta.GenderMale, meta.NewBirthday("2010-01-01")))
	assert.NoError(t, v.ValidateUpdateProfile(meta.GenderMale, meta.Birthday{}))
}
