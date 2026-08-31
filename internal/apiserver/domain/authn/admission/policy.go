package admission

import (
	"context"
	"fmt"

	loginidentitydomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Policy 是 AuthN 的认证准入策略。
// 它判断 User 与 LoginIdentity 是否允许建立或继续维持认证状态，
// 不负责资源访问授权。
type Policy interface {
	Evaluate(ctx context.Context, subject Subject) (Decision, error)
}

// LoginIdentityReader 暴露认证准入所需的最小 LoginIdentity 事实读取能力。
type LoginIdentityReader interface {
	GetByID(ctx context.Context, id meta.ID) (*loginidentitydomain.LoginIdentity, error)
}

// policy 汇集 AuthN 的 LoginIdentity 事实与 Identity 暴露的 User 状态事实。
type policy struct {
	userStatusReader    useraccess.UserStatusReader
	loginIdentityReader LoginIdentityReader
}

// NewPolicy 创建认证准入策略。
func NewPolicy(userStatusReader useraccess.UserStatusReader, loginIdentityReader LoginIdentityReader) Policy {
	return &policy{
		userStatusReader:    userStatusReader,
		loginIdentityReader: loginIdentityReader,
	}
}

// Evaluate 判断 User 与 LoginIdentity 是否允许建立或继续维持认证状态。
func (p *policy) Evaluate(ctx context.Context, subject Subject) (Decision, error) {
	if p == nil || p.loginIdentityReader == nil {
		return Decision{}, fmt.Errorf("login identity reader is not configured")
	}

	// LoginIdentity 必须存在、归属于声明的 User，且当前可用。
	identity, err := p.loginIdentityReader.GetByID(ctx, subject.LoginIdentityID)
	if err != nil {
		return Decision{}, fmt.Errorf("load login identity status: %w", err)
	}
	if identity == nil {
		return Deny(subject, ReasonLoginIdentityMissing), nil
	}
	if identity.UserID != subject.UserID {
		return Deny(subject, ReasonIdentityOwnerMismatch), nil
	}
	if !identity.IsActive() {
		return Deny(subject, ReasonLoginIdentityDisabled), nil
	}

	// User 必须存在且处于 active 状态。
	if p.userStatusReader == nil {
		return Decision{}, fmt.Errorf("identity user status reader is not configured")
	}
	userStatus, err := p.userStatusReader.ReadUserStatus(ctx, subject.UserID)
	if err != nil {
		return Decision{}, fmt.Errorf("load user status: %w", err)
	}
	switch userStatus {
	case useraccess.StatusMissing:
		return Deny(subject, ReasonUserMissing), nil
	case useraccess.StatusBlocked:
		return Deny(subject, ReasonUserBlocked), nil
	case useraccess.StatusInactive:
		return Deny(subject, ReasonUserInactive), nil
	case useraccess.StatusActive:
		return Admit(subject), nil
	default:
		return Decision{}, fmt.Errorf("identity returned unknown user status %q", userStatus)
	}
}
