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
	Evaluate(ctx context.Context, userID meta.ID, loginIdentityID meta.ID) (Decision, error)
}

type policy struct {
	userStatusReader  useraccess.UserStatusReader
	loginIdentityRepo loginidentitydomain.Repository
}

// NewPolicy 创建认证准入策略。
func NewPolicy(userStatusReader useraccess.UserStatusReader, loginIdentityRepo loginidentitydomain.Repository) Policy {
	return &policy{
		userStatusReader:  userStatusReader,
		loginIdentityRepo: loginIdentityRepo,
	}
}

// Evaluate 判断 User 与 LoginIdentity 是否允许建立或继续维持认证状态。
func (p *policy) Evaluate(ctx context.Context, userID meta.ID, loginIdentityID meta.ID) (Decision, error) {
	if p.loginIdentityRepo == nil {
		return Decision{Status: StatusDisabled, UserID: userID, LoginIdentityID: loginIdentityID}, nil
	}
	identity, err := p.loginIdentityRepo.GetByID(ctx, loginIdentityID)
	if err != nil {
		return Decision{}, fmt.Errorf("load login identity status: %w", err)
	}
	if identity == nil {
		return Decision{Status: StatusDisabled, UserID: userID, LoginIdentityID: loginIdentityID}, nil
	}
	if identity.UserID != userID {
		return Decision{Status: StatusBlocked, UserID: userID, LoginIdentityID: loginIdentityID}, nil
	}
	if !identity.Status.IsActive() {
		return Decision{Status: StatusDisabled, UserID: userID, LoginIdentityID: loginIdentityID}, nil
	}
	return p.evaluateUser(ctx, userID, loginIdentityID)
}

func (p *policy) evaluateUser(ctx context.Context, userID meta.ID, loginIdentityID meta.ID) (Decision, error) {
	if p.userStatusReader == nil {
		return Decision{}, fmt.Errorf("identity user status reader is not configured")
	}
	status, err := p.userStatusReader.ReadUserStatus(ctx, userID)
	if err != nil {
		return Decision{}, fmt.Errorf("load user status: %w", err)
	}
	switch status {
	case useraccess.StatusMissing, useraccess.StatusBlocked:
		return Decision{Status: StatusBlocked, UserID: userID, LoginIdentityID: loginIdentityID}, nil
	case useraccess.StatusInactive:
		return Decision{Status: StatusInactive, UserID: userID, LoginIdentityID: loginIdentityID}, nil
	case useraccess.StatusActive:
		// Continue below.
	default:
		return Decision{}, fmt.Errorf("identity returned unknown user status %q", status)
	}

	return Decision{
		Status:          StatusActive,
		UserID:          userID,
		LoginIdentityID: loginIdentityID,
	}, nil
}
