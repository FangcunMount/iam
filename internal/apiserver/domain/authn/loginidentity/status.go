package loginidentity

// Status 登录身份状态
type Status string

const (
	StatusDisabled Status = "disabled" // 禁用
	StatusActive   Status = "active"   // 活动
	StatusArchived Status = "archived" // 归档
	StatusDeleted  Status = "deleted"  // 删除
)

// Validate 验证状态是否有效
func (s Status) Validate() bool {
	switch s {
	case StatusDisabled, StatusActive, StatusArchived, StatusDeleted:
		return true
	default:
		return false
	}
}

// IsActive 是否活动
func (s Status) IsActive() bool { return s == StatusActive }
