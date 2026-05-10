package role

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
)

func TestNewRole(t *testing.T) {
	id := meta.FromUint64(10)
	r, err := NewRole("admin", "管理员", "tenant1", WithID(id), WithDescription("desc"))
	assert.NoError(t, err)
	assert.Equal(t, "admin", r.Name)
	assert.Equal(t, "管理员", r.DisplayName)
	assert.Equal(t, "desc", r.Description)
}

func TestRoleDomainBehavior(t *testing.T) {
	r, err := NewRole("admin", "管理员", "tenant1")
	assert.NoError(t, err)
	assert.True(t, r.BelongsToTenant("tenant1"))
	assert.False(t, r.BelongsToTenant("tenant2"))

	err = r.Rename("系统管理员")
	assert.NoError(t, err)
	assert.Equal(t, "系统管理员", r.DisplayName)

	err = r.Rename("")
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	r.ChangeDescription("new desc")
	assert.Equal(t, "new desc", r.Description)
}

func TestNewRoleRejectsInvalidState(t *testing.T) {
	_, err := NewRole("", "管理员", "tenant1")
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	_, err = NewRole("admin", "", "tenant1")
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	_, err = NewRole("admin", "管理员", "")
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}
