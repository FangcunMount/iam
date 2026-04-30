package dto

// CheckRequest PDP 判定请求。
type CheckRequest struct {
	Object     string `json:"object" binding:"required"`
	Action     string `json:"action" binding:"required"`
	ScopeType  string `json:"scope_type"`
	ScopeValue string `json:"scope_value"`
	// SubjectType 可选：user | group；与 SubjectID 同时省略时使用当前 JWT 用户。
	SubjectType string `json:"subject_type"`
	SubjectID   string `json:"subject_id"`
}

// CheckResponse PDP 判定结果。
type CheckResponse struct {
	Allowed bool `json:"allowed"`
}
