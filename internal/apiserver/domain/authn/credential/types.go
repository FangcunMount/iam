package credential

import "time"

// CredentialType 凭据类型
type CredentialType string

const (
	CredPassword CredentialType = "password"
)

// CredentialStatus 凭据状态
type CredentialStatus int8

const (
	CredStatusDisabled CredentialStatus = 0 // 禁用
	CredStatusEnabled  CredentialStatus = 1 // 启用
)

func (s CredentialStatus) String() string {
	switch s {
	case CredStatusDisabled:
		return "disabled"
	case CredStatusEnabled:
		return "enabled"
	default:
		return "unknown"
	}
}

// Validate 校验凭据状态是否合法
func (s CredentialStatus) Validate() bool {
	return s >= CredStatusDisabled && s <= CredStatusEnabled
}

// LockoutPolicy 锁定策略
type LockoutPolicy struct {
	Enabled      bool
	Threshold    int
	LockDuration time.Duration
}
