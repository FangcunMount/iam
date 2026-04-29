package request

// ChildCreateRequest 创建儿童档案请求（身份证可选）
type ChildCreateRequest struct {
	LegalName string `json:"legalName" binding:"required"`
	Gender    *uint8 `json:"gender" binding:"required"`
	DOB       string `json:"dob" binding:"required"`
	IDType    string `json:"idType,omitempty"`
	IDNo      string `json:"idNo,omitempty"`
	HeightCm  *int   `json:"heightCm,omitempty"`
	WeightKg  string `json:"weightKg,omitempty"`
}

// ChildRegisterRequest 注册儿童档案并授予监护
type ChildRegisterRequest struct {
	ChildCreateRequest
	Relation string `json:"relation" binding:"required,oneof=self parent grandparent other"`
}

// ChildUpdateRequest 更新儿童档案请求
type ChildUpdateRequest struct {
	LegalName *string `json:"legalName,omitempty"`
	Gender    *uint8  `json:"gender,omitempty"`
	DOB       *string `json:"dob,omitempty"`
	HeightCm  *int    `json:"heightCm,omitempty"`
	WeightKg  *string `json:"weightKg,omitempty"`
}

// ChildSearchQuery 搜索孩子请求参数
type ChildSearchQuery struct {
	Name   string  `form:"name"`
	DOB    *string `form:"dob"`
	Limit  int     `form:"limit,default=20"`
	Offset int     `form:"offset,default=0"`
}

// ChildListQuery 列表查询通用参数
type ChildListQuery struct {
	Limit  int `form:"limit,default=20"`
	Offset int `form:"offset,default=0"`
}
