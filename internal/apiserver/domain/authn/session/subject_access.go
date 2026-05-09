package session

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// SubjectAccessStatus 表示认证主体的可访问状态。
type SubjectAccessStatus string

const (
	SubjectAccessActive   SubjectAccessStatus = "active"
	SubjectAccessBlocked  SubjectAccessStatus = "blocked"
	SubjectAccessDisabled SubjectAccessStatus = "disabled"
	SubjectAccessLocked   SubjectAccessStatus = "locked"
)

// SubjectAccessDecision 汇总 user/login identity 的访问判定。
type SubjectAccessDecision struct {
	Status          SubjectAccessStatus
	UserID          meta.ID
	LoginIdentityID meta.ID
}

// IsAllowed 返回当前主体是否允许继续访问。
func (d SubjectAccessDecision) IsAllowed() bool {
	return d.Status == SubjectAccessActive
}

// SubjectAccessEvaluator 负责汇总 user/login identity 访问状态。
type SubjectAccessEvaluator interface {
	Evaluate(ctx context.Context, userID meta.ID, loginIdentityID meta.ID) (SubjectAccessDecision, error)
}
