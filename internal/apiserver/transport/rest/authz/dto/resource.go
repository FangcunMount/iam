// Package dto 资源相关的 DTO 定义
package dto

import "github.com/FangcunMount/iam/internal/pkg/meta"

// CreateResourceRequest 创建资源请求
type CreateResourceRequest struct {
	Key         string   `json:"key" binding:"required"`
	DisplayName string   `json:"display_name" binding:"required"`
	AppName     string   `json:"app_name" binding:"required"`
	Domain      string   `json:"domain" binding:"required"`
	Type        string   `json:"type" binding:"required"`
	Actions     []string `json:"actions" binding:"required,min=1"`
	Description string   `json:"description"`
}

// UpdateResourceRequest 更新资源请求
type UpdateResourceRequest struct {
	DisplayName string   `json:"display_name"`
	Actions     []string `json:"actions" binding:"min=1"`
	Description string   `json:"description"`
}

// ResourceResponse 资源响应
type ResourceResponse struct {
	ID          meta.ID  `json:"id" swaggertype:"string"`
	Key         string   `json:"key"`
	DisplayName string   `json:"display_name"`
	AppName     string   `json:"app_name"`
	Domain      string   `json:"domain"`
	Type        string   `json:"type"`
	Actions     []string `json:"actions"`
	Description string   `json:"description"`
}

// ListResourceQuery 列出资源查询参数
type ListResourceQuery struct {
	Offset int `form:"offset"`
	Limit  int `form:"limit"`
}

// ValidateActionRequest 验证动作请求
type ValidateActionRequest struct {
	ResourceKey string `json:"resource_key" binding:"required"`
	Action      string `json:"action" binding:"required"`
}

// ValidateActionResponse 验证动作响应
type ValidateActionResponse struct {
	Valid bool `json:"valid"`
}
