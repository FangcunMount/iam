// Package dto 策略相关的 DTO 定义
package dto

import "github.com/FangcunMount/iam/v3/internal/pkg/meta"

// AddPermissionRequest 添加权限请求
type AddPermissionRequest struct {
	RoleID     meta.ID `json:"role_id" binding:"required" swaggertype:"string"`
	ResourceID meta.ID `json:"resource_id" binding:"required" swaggertype:"string"`
	Action     string  `json:"action" binding:"required"`
	ScopeType  string  `json:"scope_type"`
	ScopeValue string  `json:"scope_value"`
	ChangedBy  string  `json:"changed_by,omitempty"`
	Reason     string  `json:"reason"`
}

// RemovePermissionRequest 移除权限请求
type RemovePermissionRequest struct {
	RoleID     meta.ID `json:"role_id" binding:"required" swaggertype:"string"`
	ResourceID meta.ID `json:"resource_id" binding:"required" swaggertype:"string"`
	Action     string  `json:"action" binding:"required"`
	ScopeType  string  `json:"scope_type"`
	ScopeValue string  `json:"scope_value"`
	ChangedBy  string  `json:"changed_by,omitempty"`
	Reason     string  `json:"reason"`
}

// PermissionResponse 权限响应
type PermissionResponse struct {
	Subject    string `json:"subject"`
	Domain     string `json:"domain"`
	Object     string `json:"object"`
	Action     string `json:"action"`
	ScopeType  string `json:"scope_type"`
	ScopeValue string `json:"scope_value"`
}

// PolicyVersionResponse 策略版本响应
type PolicyVersionResponse struct {
	TenantID  string `json:"tenant_id"`
	Version   int64  `json:"version"`
	ChangedBy string `json:"changed_by"`
	Reason    string `json:"reason"`
}

type PolicyLintResponse struct {
	Findings []PolicyLintFindingResponse `json:"findings"`
}

type PolicyLintFindingResponse struct {
	Code        string `json:"code"`
	RoleName    string `json:"role_name"`
	TenantID    string `json:"tenant_id"`
	ResourceKey string `json:"resource_key"`
	Action      string `json:"action"`
	Scope       string `json:"scope"`
	Message     string `json:"message"`
}
