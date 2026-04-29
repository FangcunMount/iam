package account

// Provider 认证提供者
type Provider string

const (
	ProviderPassword Provider = "op:password"
	ProviderWeChat   Provider = "wx:minip"
)

// AppId 应用 ID
type AppId string

// Len returns the length of the AppId string
func (a AppId) Len() int {
	return len(a)
}

// ExternalID 外部平台用户标识
type ExternalID string

// Len returns the length of the ExternalID string
func (e ExternalID) Len() int {
	return len(e)
}

// UnionID 全局唯一标识
type UnionID string

// Len returns the length of the UnionID string
func (u UnionID) Len() int {
	return len(u)
}

// AccountType 账号类型
type AccountType string

const (
	TypeWcMinip      AccountType = "wc-minip"      // 微信小程序
	TypeWcOffi       AccountType = "wc-offi"       // 微信公众号
	TypeWcCom        AccountType = "wc-com"        // 企业微信
	TypeOpera        AccountType = "opera"         // 运营后台
	TypeMockConsumer AccountType = "mock-consumer" // 内部 mock C 端
)

const (
	// MockConsumerAppID 是内部 mock C 端账号固定使用的应用 ID。
	// 现网 auth_accounts.app_id 仍按非空字段建模，显式设置固定值可避免
	// NULL/空字符串在不同 schema 或查询分支下造成的兼容性问题。
	MockConsumerAppID AppId = "mock-consumer"
)

// String 转字符串
func (a AccountType) String() string {
	return string(a)
}

// Validate 校验账号类型是否合法
func (a AccountType) Validate() bool {
	tList := []AccountType{TypeWcMinip, TypeWcOffi, TypeWcCom, TypeOpera, TypeMockConsumer}
	for _, t := range tList {
		if a == t {
			return true
		}
	}
	return false
}

// Status 账号状态
type AccountStatus int8

const (
	StatusDisabled AccountStatus = 0 // 禁用
	StatusActive   AccountStatus = 1 // 激活
	StatusArchived AccountStatus = 2 // 已归档
	StatusDeleted  AccountStatus = 3 // 已删除
)

func (s AccountStatus) String() string {
	switch s {
	case StatusDisabled:
		return "disabled"
	case StatusActive:
		return "active"
	case StatusArchived:
		return "archived"
	case StatusDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}
