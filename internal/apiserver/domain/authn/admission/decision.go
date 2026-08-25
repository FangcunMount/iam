package admission

import "github.com/FangcunMount/iam/v3/internal/pkg/meta"

// Status 表示认证主体的准入状态。
type Status string

const (
	StatusActive   Status = "active"   // User 与 LoginIdentity 均允许建立或维持认证状态。
	StatusBlocked  Status = "blocked"  // User 不存在、已封禁，或 LoginIdentity 不属于该 User。
	StatusDisabled Status = "disabled" // LoginIdentity 不存在或已禁用。
	StatusInactive Status = "inactive" // User 尚未激活。
)

// Decision 表示 User 与 LoginIdentity 的认证准入判定。
type Decision struct {
	Status          Status
	UserID          meta.ID
	LoginIdentityID meta.ID
}

// IsAdmitted 返回当前身份组合是否允许建立或维持认证状态。
func (d Decision) IsAdmitted() bool {
	return d.Status == StatusActive
}
