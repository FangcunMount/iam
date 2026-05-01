// Package dto 赋权相关的 DTO 定义
package dto

import "github.com/FangcunMount/iam/v2/internal/pkg/meta"

// GrantRequest 授权请求
type GrantRequest struct {
	SubjectType string  `json:"subject_type" binding:"required,oneof=user"`
	SubjectID   string  `json:"subject_id" binding:"required"`
	RoleID      meta.ID `json:"role_id" binding:"required" swaggertype:"string"`
	GrantedBy   string  `json:"granted_by,omitempty"`
}

// RevokeRequest 撤销授权请求
type RevokeRequest struct {
	SubjectType string  `json:"subject_type" binding:"required,oneof=user"`
	SubjectID   string  `json:"subject_id" binding:"required"`
	RoleID      meta.ID `json:"role_id" binding:"required" swaggertype:"string"`
}

// AssignmentResponse 赋权响应
type AssignmentResponse struct {
	ID          meta.ID `json:"id" swaggertype:"string"`
	SubjectType string  `json:"subject_type"`
	SubjectID   string  `json:"subject_id"`
	RoleID      meta.ID `json:"role_id" swaggertype:"string"`
	TenantID    string  `json:"tenant_id"`
	GrantedBy   string  `json:"granted_by"`
}

// ListAssignmentQuery 列出赋权查询参数
type ListAssignmentQuery struct {
	SubjectType string  `form:"subject_type"`
	SubjectID   string  `form:"subject_id"`
	RoleID      meta.ID `form:"role_id" swaggertype:"string"`
}
