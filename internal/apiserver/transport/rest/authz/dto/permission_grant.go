package dto

import (
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

type CreatePermissionGrantRequest struct {
	RoleID        meta.ID        `json:"role_id" binding:"required" swaggertype:"string"`
	ResourceID    meta.ID        `json:"resource_id" binding:"required" swaggertype:"string"`
	Action        string         `json:"action" binding:"required"`
	ConstraintSet constraint.Set `json:"constraint_set"`
}

type RevokePermissionGrantRequest struct {
	Reason string `json:"reason"`
}

type PermissionGrantResponse struct {
	ID              meta.ID        `json:"id" swaggertype:"string"`
	TenantID        string         `json:"tenant_id"`
	RoleID          meta.ID        `json:"role_id" swaggertype:"string"`
	ResourceID      meta.ID        `json:"resource_id" swaggertype:"string"`
	ResourcePattern string         `json:"resource_pattern"`
	Action          string         `json:"action"`
	ConstraintSet   constraint.Set `json:"constraint_set"`
	GrantKey        string         `json:"grant_key"`
	GrantedBy       string         `json:"granted_by"`
	Active          bool           `json:"active"`
}
