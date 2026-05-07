package user

// Status 用户状态
type Status uint8

const (
	UserActive   Status = 1 + iota // 1：活跃
	UserInactive                   // 2：非活跃
	UserBlocked                    // 3：被封禁
)

// Uint64 获取 Uint64 类型值
func (s Status) Uint64() uint64 {
	return uint64(s)
}

// Value 获取状态值
func (s Status) Value() uint8 {
	return uint8(s)
}

// String 获取状态字符串
func (s Status) String() string {
	switch s {
	case UserActive:
		return "active"
	case UserInactive:
		return "inactive"
	case UserBlocked:
		return "blocked"
	default:
		return "unknown"
	}
}

// IsValid 校验 Status 值的合法性
func (s Status) IsValid() bool {
	switch s {
	case UserActive, UserInactive, UserBlocked:
		return true
	default:
		return false
	}
}
