package request

// ProfileLinkCreateRequest 授予关系请求
type ProfileLinkCreateRequest struct {
	UserID    string `json:"userId"`
	ProfileID string `json:"profileId" binding:"required"`
	Relation  string `json:"relation" binding:"required,oneof=self parent grandparent other"`
}

// ProfileLinkRevokeRequest 撤销关系请求
type ProfileLinkRevokeRequest struct {
	UserID    string `json:"userId" binding:"required"`
	ProfileID string `json:"profileId" binding:"required"`
}

// ProfileLinkListQuery 档案关系查询参数
type ProfileLinkListQuery struct {
	UserID    string `form:"user_id"`
	ProfileID string `form:"profile_id"`
	Active    *bool  `form:"active"`
	Limit     int    `form:"limit,default=20"`
	Offset    int    `form:"offset,default=0"`
}
