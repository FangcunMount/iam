package request

// GuardianGrantRequest 授予监护请求
type GuardianGrantRequest struct {
	UserID   string `json:"userId"`
	ChildID  string `json:"childId" binding:"required"`
	Relation string `json:"relation" binding:"required,oneof=self parent grandparent other"`
}

// GuardianRevokeRequest 撤销监护请求
type GuardianRevokeRequest struct {
	UserID  string `json:"userId" binding:"required"`
	ChildID string `json:"childId" binding:"required"`
}

// GuardianshipListQuery 监护关系查询参数
type GuardianshipListQuery struct {
	UserID  string `form:"user_id"`
	ChildID string `form:"child_id"`
	Active  *bool  `form:"active"`
	Limit   int    `form:"limit,default=20"`
	Offset  int    `form:"offset,default=0"`
}
