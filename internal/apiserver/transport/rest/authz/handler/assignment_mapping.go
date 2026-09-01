package handler

import (
	assignmentDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authz/dto"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// toAssignmentResponse 转换为响应对象
func (h *AssignmentHandler) toAssignmentResponse(a *assignmentDomain.Assignment) dto.AssignmentResponse {
	return dto.AssignmentResponse{
		ID:          meta.ID(a.ID),
		SubjectType: a.SubjectTypeString(),
		SubjectID:   a.SubjectID,
		RoleID:      a.RoleID,
		TenantID:    a.TenantIDString(),
		GrantedBy:   a.GrantedBy,
	}
}
