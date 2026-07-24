package request

// ProfileUpdateRequest 更新档案请求
type ProfileUpdateRequest struct {
	LegalName *string `json:"legalName,omitempty"`
	Gender    *uint8  `json:"gender,omitempty"`
	DOB       *string `json:"dob,omitempty"`
}

// ProfileListQuery 列表查询通用参数
type ProfileListQuery struct {
	Limit  int `form:"limit,default=20"`
	Offset int `form:"offset,default=0"`
}
