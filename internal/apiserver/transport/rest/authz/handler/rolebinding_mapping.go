package handler

import (
	bindingDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authz/dto"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// toAssignmentResponse 转换为响应对象
func (h *RoleBindingHandler) toAssignmentResponse(a *bindingDomain.Binding) dto.AssignmentResponse {
	return dto.AssignmentResponse{
		ID:          meta.ID(a.ID),
		SubjectType: a.SubjectTypeString(),
		SubjectID:   a.SubjectID,
		RoleID:      a.RoleID,
		TenantID:    a.TenantIDString(),
		GrantedBy:   a.GrantedBy,
	}
}
