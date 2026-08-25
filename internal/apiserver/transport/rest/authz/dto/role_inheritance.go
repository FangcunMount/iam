package dto

import (
	"time"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

type CreateRoleInheritanceRequest struct {
	RoleID          meta.ID `json:"role_id" binding:"required" swaggertype:"string"`
	InheritedRoleID meta.ID `json:"inherited_role_id" binding:"required" swaggertype:"string"`
}

type RevokeRoleInheritanceRequest struct {
	Reason string `json:"reason"`
}

type ListRoleInheritanceQuery struct {
	RoleID meta.ID `form:"role_id" swaggertype:"string"`
}

type RoleInheritanceResponse struct {
	ID              meta.ID   `json:"id" swaggertype:"string"`
	TenantID        string    `json:"tenant_id"`
	RoleID          meta.ID   `json:"role_id" swaggertype:"string"`
	InheritedRoleID meta.ID   `json:"inherited_role_id" swaggertype:"string"`
	GrantedBy       string    `json:"granted_by"`
	GrantedAt       time.Time `json:"granted_at"`
	Active          bool      `json:"active"`
}
