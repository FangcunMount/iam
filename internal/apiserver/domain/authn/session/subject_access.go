package session

import (
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// SubjectAccessStatus 表示认证主体的可访问状态。
type SubjectAccessStatus string

const (
	SubjectAccessActive   SubjectAccessStatus = "active"   // 用户/登录身份处于活跃状态。
	SubjectAccessBlocked  SubjectAccessStatus = "blocked"  // 用户/登录身份处于封禁状态。
	SubjectAccessDisabled SubjectAccessStatus = "disabled" // 用户/登录身份处于禁用状态。
)

// SubjectAccessDecision 表示用户/登录身份的访问状态。
type SubjectAccessDecision struct {
	Status          SubjectAccessStatus
	UserID          meta.ID
	LoginIdentityID meta.ID
}

// IsAllowed 返回用户/登录身份是否允许继续访问。
func (d SubjectAccessDecision) IsAllowed() bool {
	return d.Status == SubjectAccessActive
}
