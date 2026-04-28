package role

import (
	"testing"

	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
)

func TestNewRoleAndKey(t *testing.T) {
	id := meta.FromUint64(10)
	r := NewRole("admin", "管理员", "tenant1", WithID(id), WithDescription("desc"))
	assert.Equal(t, "admin", r.Name)
	assert.Equal(t, "管理员", r.DisplayName)
	assert.Equal(t, "desc", r.Description)
	assert.Equal(t, "role:admin", r.Key())
}
