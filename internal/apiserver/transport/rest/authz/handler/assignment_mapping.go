package handler

import (
	assignmentDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/internal/apiserver/transport/rest/authz/dto"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// toAssignmentResponse 转换为响应对象
func (h *AssignmentHandler) toAssignmentResponse(a *assignmentDomain.Assignment) dto.AssignmentResponse {
	return dto.AssignmentResponse{
		ID:          meta.ID(a.ID),
		SubjectType: a.SubjectType.String(),
		SubjectID:   a.SubjectID,
		RoleID:      meta.FromUint64(a.RoleID),
		TenantID:    a.TenantID,
		GrantedBy:   a.GrantedBy,
	}
}
