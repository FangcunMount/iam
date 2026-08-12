// Package dto 角色相关的 DTO 定义
package dto

import "github.com/FangcunMount/iam/v3/internal/pkg/meta"

// CreateRoleRequest 创建角色请求
type CreateRoleRequest struct {
	Name        string `json:"name" binding:"required"`
	DisplayName string `json:"display_name" binding:"required"`
	Description string `json:"description"`
}

// UpdateRoleRequest 更新角色请求
type UpdateRoleRequest struct {
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

// RoleResponse 角色响应
type RoleResponse struct {
	ID          meta.ID `json:"id" swaggertype:"string"`
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	TenantID    string  `json:"tenant_id"`
	Description string  `json:"description"`
}

// ListRoleQuery 列出角色查询参数
type ListRoleQuery struct {
	Offset int `form:"offset"`
	Limit  int `form:"limit"`
}
