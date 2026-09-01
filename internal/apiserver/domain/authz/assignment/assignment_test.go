package assignment_test

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	assignment "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssignmentCreate(t *testing.T) {
	a, err := assignment.NewAssignment(assignment.SubjectTypeUser, meta.FromUint64(1), meta.FromUint64(42), "tenant", assignment.WithID(assignment.NewAssignmentID(5)), assignment.WithGrantedBy("admin"))
	require.NoError(t, err)
	assert.Equal(t, assignment.SubjectTypeUser, a.SubjectType)
	assert.Equal(t, meta.FromUint64(1), a.SubjectID)
	assert.Equal(t, "admin", a.GrantedBy)
}

func TestAssignmentCreateRejectsInvalidState(t *testing.T) {
	tests := []struct {
		name        string
		subjectType assignment.SubjectType
		subjectID   meta.ID
		roleID      meta.ID
		tenantID    string
		grantedBy   string
	}{
		{name: "unsupported subject type", subjectType: "robot", subjectID: meta.FromUint64(1), roleID: meta.FromUint64(2), tenantID: "tenant", grantedBy: "admin"},
		{name: "missing subject id", subjectType: assignment.SubjectTypeUser, roleID: meta.FromUint64(2), tenantID: "tenant", grantedBy: "admin"},
		{name: "missing role id", subjectType: assignment.SubjectTypeUser, subjectID: meta.FromUint64(1), tenantID: "tenant", grantedBy: "admin"},
		{name: "missing tenant", subjectType: assignment.SubjectTypeUser, subjectID: meta.FromUint64(1), roleID: meta.FromUint64(2), grantedBy: "admin"},
		{name: "missing granted by", subjectType: assignment.SubjectTypeUser, subjectID: meta.FromUint64(1), roleID: meta.FromUint64(2), tenantID: "tenant"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := assignment.NewAssignment(tt.subjectType, tt.subjectID, tt.roleID, tt.tenantID, assignment.WithGrantedBy(tt.grantedBy))
			assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
		})
	}
}
