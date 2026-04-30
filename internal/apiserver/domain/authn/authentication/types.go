package authentication

import "github.com/FangcunMount/iam/internal/pkg/meta"

// AMR（认证方法引用），用于审计与 Step-Up
type AMR string

const (
	AMRPassword AMR = "pwd"
	AMROTP      AMR = "otp"
	AMRWx       AMR = "wechat"
	AMRWecom    AMR = "wecom"
)

// 认证主体（输出，用于签 Token/授权）
type Principal struct {
	AccountID meta.ID
	UserID    meta.ID
	TenantID  meta.ID
	SessionID string
	AMR       []string
	Claims    map[string]any
}
