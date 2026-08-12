package dto

import "github.com/FangcunMount/iam/v3/internal/pkg/meta"

// CheckRequest PDP 判定请求。
type CheckRequest struct {
	Object     string `json:"object" binding:"required"`
	Action     string `json:"action" binding:"required"`
	ScopeType  string `json:"scope_type"`
	ScopeValue string `json:"scope_value"`
	// SubjectType 可选：user | group；与 SubjectID 同时省略时使用当前 JWT 用户。
	SubjectType string  `json:"subject_type"`
	SubjectID   meta.ID `json:"subject_id" swaggertype:"string"`
}

// CheckResponse PDP 判定结果。
type CheckResponse struct {
	Allowed       bool   `json:"allowed"`
	Reason        string `json:"reason,omitempty"`
	DenyCode      string `json:"deny_code,omitempty"`
	PolicyVersion int64  `json:"policy_version,omitempty"`
}
