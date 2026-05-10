package rolebinding_test

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	binding "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBindingCreate(t *testing.T) {
	a, err := binding.NewBinding(binding.SubjectTypeUser, meta.FromUint64(1), meta.FromUint64(42), "tenant", binding.WithID(binding.NewBindingID(5)), binding.WithGrantedBy("admin"))
	require.NoError(t, err)
	assert.Equal(t, binding.SubjectTypeUser, a.SubjectType)
	assert.Equal(t, meta.FromUint64(1), a.SubjectID)
	assert.Equal(t, "admin", a.GrantedBy)
}

func TestBindingCreateRejectsInvalidState(t *testing.T) {
	tests := []struct {
		name        string
		subjectType binding.SubjectType
		subjectID   meta.ID
		roleID      meta.ID
		tenantID    string
		grantedBy   string
	}{
		{name: "unsupported subject type", subjectType: "robot", subjectID: meta.FromUint64(1), roleID: meta.FromUint64(2), tenantID: "tenant", grantedBy: "admin"},
		{name: "missing subject id", subjectType: binding.SubjectTypeUser, roleID: meta.FromUint64(2), tenantID: "tenant", grantedBy: "admin"},
		{name: "missing role id", subjectType: binding.SubjectTypeUser, subjectID: meta.FromUint64(1), tenantID: "tenant", grantedBy: "admin"},
		{name: "missing tenant", subjectType: binding.SubjectTypeUser, subjectID: meta.FromUint64(1), roleID: meta.FromUint64(2), grantedBy: "admin"},
		{name: "missing granted by", subjectType: binding.SubjectTypeUser, subjectID: meta.FromUint64(1), roleID: meta.FromUint64(2), tenantID: "tenant"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := binding.NewBinding(tt.subjectType, tt.subjectID, tt.roleID, tt.tenantID, binding.WithGrantedBy(tt.grantedBy))
			assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
		})
	}
}
