package credential

import (
	"time"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// AuthenticationTransitionKind 认证状态迁移类型。
type AuthenticationTransitionKind int

const (
	TransitionNone AuthenticationTransitionKind = iota
	TransitionRecordFailure
	TransitionRecordSuccess
)

// AuthenticationTransition 描述一次凭据认证状态迁移意图。
type AuthenticationTransition struct {
	CredentialID  meta.ID
	Kind          AuthenticationTransitionKind
	Now           time.Time
	LockoutPolicy LockoutPolicy
	Rotation      *MaterialRotation
}

// ApplyAuthenticationTransition 将迁移意图应用到实体，返回迁移后的认证状态摘要。
func ApplyAuthenticationTransition(c *Credential, transition AuthenticationTransition) AuthenticationState {
	if c == nil || transition.Kind == TransitionNone {
		return AuthenticationState{}
	}
	now := transition.Now
	switch transition.Kind {
	case TransitionRecordFailure:
		c.RecordFailure(now)
		newlyLocked := c.ApplyLockPolicy(now, transition.LockoutPolicy)
		return AuthenticationState{
			FailedAttempts: c.FailedAttempts,
			LockedUntil:    c.LockedUntil,
			NewlyLocked:    newlyLocked,
		}
	case TransitionRecordSuccess:
		c.RecordSuccess(now)
		if transition.Rotation != nil && len(transition.Rotation.Material) > 0 {
			c.RotateMaterial(transition.Rotation.Material, transition.Rotation.Algo)
		}
		return AuthenticationState{}
	default:
		return AuthenticationState{}
	}
}

// NewFailureTransition 构造失败计数迁移。
func NewFailureTransition(credentialID meta.ID, now time.Time, policy LockoutPolicy) AuthenticationTransition {
	return AuthenticationTransition{
		CredentialID:  credentialID,
		Kind:          TransitionRecordFailure,
		Now:           now,
		LockoutPolicy: policy,
	}
}

// NewSuccessTransition 构造成功迁移，可携带材料轮换。
func NewSuccessTransition(credentialID meta.ID, now time.Time, rotation *MaterialRotation) AuthenticationTransition {
	return AuthenticationTransition{
		CredentialID: credentialID,
		Kind:         TransitionRecordSuccess,
		Now:          now,
		Rotation:     rotation,
	}
}
