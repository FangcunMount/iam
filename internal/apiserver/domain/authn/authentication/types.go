package authentication

import "github.com/FangcunMount/iam/v2/internal/pkg/meta"

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
	LoginIdentityID meta.ID
	UserID          meta.ID
	TenantID        meta.ID
	SessionID       string
	AuthMethod      string
	Realm           string
	AMR             []string
	Claims          map[string]any
}
