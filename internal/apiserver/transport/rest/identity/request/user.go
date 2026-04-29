package request

// UserContactUpsert 表示用户联系方式的写入结构
type UserContactUpsert struct {
	Type  string `json:"type" binding:"required,oneof=email phone"`
	Value string `json:"value" binding:"required"`
}

// UserCreateRequest 创建用户请求
type UserCreateRequest struct {
	Nickname string              `json:"nickname,omitempty"`
	Avatar   string              `json:"avatar,omitempty"`
	Contacts []UserContactUpsert `json:"contacts,omitempty"`
}

// UserUpdateRequest 更新用户请求
type UserUpdateRequest struct {
	Nickname *string             `json:"nickname,omitempty"`
	Avatar   *string             `json:"avatar,omitempty"`
	Contacts []UserContactUpsert `json:"contacts,omitempty"`
}
