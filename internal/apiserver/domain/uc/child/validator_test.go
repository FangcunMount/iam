package child_test

import (
	"context"
	"fmt"
	"testing"

	child "github.com/FangcunMount/iam/internal/apiserver/domain/uc/child"
	"github.com/FangcunMount/iam/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChildValidator_ValidateRegister(t *testing.T) {
	v := child.NewValidator(&testhelpers.ChildRepoStub{})
	err := v.ValidateRegister(context.Background(), "name", meta.GenderMale, meta.NewBirthday("2010-01-01"))
	assert.NoError(t, err)
}

func TestChildValidator_ValidateRegister_EmptyName(t *testing.T) {
	v := child.NewValidator(&testhelpers.ChildRepoStub{})
	err := v.ValidateRegister(context.Background(), "", meta.GenderMale, meta.NewBirthday("2010-01-01"))
	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "name cannot be empty")
}

func TestChildValidator_ValidateRegister_EmptyBirthday(t *testing.T) {
	v := child.NewValidator(&testhelpers.ChildRepoStub{})
	err := v.ValidateRegister(context.Background(), "name", meta.GenderMale, meta.Birthday{})
	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "birthday cannot be empty")
}

func TestChildValidator_ValidateRename(t *testing.T) {
	v := child.NewValidator(&testhelpers.ChildRepoStub{})
	assert.NoError(t, v.ValidateRename("valid"))

	err := v.ValidateRename("")
	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "name cannot be empty")
}

func TestChildValidator_ValidateUpdateProfile(t *testing.T) {
	v := child.NewValidator(&testhelpers.ChildRepoStub{})
	assert.NoError(t, v.ValidateUpdateProfile(meta.GenderMale, meta.NewBirthday("2010-01-01")))

	err := v.ValidateUpdateProfile(meta.GenderMale, meta.Birthday{})
	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "birthday cannot be empty")
}
