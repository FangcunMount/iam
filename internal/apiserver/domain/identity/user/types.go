package user

// UserStatus 用户状态
type UserStatus uint8

func (s UserStatus) Uint64() uint64 {
	return uint64(s)
}

const (
	UserActive   UserStatus = 1 + iota // 1：活跃
	UserInactive                       // 2：非活跃
	UserBlocked                        // 3：被封禁
)

// Value 获取状态值
func (s UserStatus) Value() uint8 {
	return uint8(s)
}

// String 获取状态字符串
func (s UserStatus) String() string {
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
