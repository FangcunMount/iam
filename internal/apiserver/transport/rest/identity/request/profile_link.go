package request

// ProfileLinkListQuery 档案关系查询参数
type ProfileLinkListQuery struct {
	UserID         string `form:"user_id"`
	ProfileID      string `form:"profile_id"`
	IncludeRevoked *bool  `form:"include_revoked"`
	Active         *bool  `form:"active"`
	Limit          int    `form:"limit,default=20"`
	Offset         int    `form:"offset,default=0"`
}
