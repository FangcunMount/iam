package handler

import (
	roleDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/internal/apiserver/transport/rest/authz/dto"
)

// toRoleResponse 转换为响应对象
func (h *RoleHandler) toRoleResponse(r *roleDomain.Role) dto.RoleResponse {
	return dto.RoleResponse{
		ID:          r.ID,
		Name:        r.Name,
		DisplayName: r.DisplayName,
		TenantID:    r.TenantID,
		Description: r.Description,
	}
}
