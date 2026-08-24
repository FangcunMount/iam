package session

import (
	"context"
	"fmt"

	loginidentitydomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// subjectAccessEvaluator 用于评估用户/登录身份的访问状态。
type subjectAccessEvaluator struct {
	userStatusReader  useraccess.UserStatusReader    // Identity 用户状态窄能力
	loginIdentityRepo loginidentitydomain.Repository // 登录身份仓库
}

// NewSubjectAccessEvaluator 创建用户/登录身份访问状态评估器。
func NewSubjectAccessEvaluator(userStatusReader useraccess.UserStatusReader, loginIdentityRepo loginidentitydomain.Repository) SubjectAccessEvaluator {
	return &subjectAccessEvaluator{
		userStatusReader:  userStatusReader,
		loginIdentityRepo: loginIdentityRepo,
	}
}

// Evaluate 评估用户/登录身份的访问状态，返回访问状态决策。
func (e *subjectAccessEvaluator) Evaluate(ctx context.Context, userID meta.ID, loginIdentityID meta.ID) (SubjectAccessDecision, error) {
	if e.loginIdentityRepo == nil {
		return SubjectAccessDecision{Status: SubjectAccessDisabled, UserID: userID, LoginIdentityID: loginIdentityID}, nil
	}
	identity, err := e.loginIdentityRepo.GetByID(ctx, loginIdentityID)
	if err != nil {
		return SubjectAccessDecision{}, fmt.Errorf("load login identity status: %w", err)
	}
	if identity == nil {
		return SubjectAccessDecision{Status: SubjectAccessDisabled, UserID: userID, LoginIdentityID: loginIdentityID}, nil
	}
	if identity.UserID != userID {
		return SubjectAccessDecision{Status: SubjectAccessBlocked, UserID: userID, LoginIdentityID: loginIdentityID}, nil
	}
	if !identity.Status.IsActive() {
		return SubjectAccessDecision{Status: SubjectAccessDisabled, UserID: userID, LoginIdentityID: loginIdentityID}, nil
	}
	return e.evaluateUser(ctx, userID, loginIdentityID)
}

// evaluateUser 评估用户的状态。
func (e *subjectAccessEvaluator) evaluateUser(ctx context.Context, userID meta.ID, loginIdentityID meta.ID) (SubjectAccessDecision, error) {
	if e.userStatusReader == nil {
		return SubjectAccessDecision{}, fmt.Errorf("identity user status reader is not configured")
	}
	status, err := e.userStatusReader.ReadUserStatus(ctx, userID)
	if err != nil {
		return SubjectAccessDecision{}, fmt.Errorf("load user status: %w", err)
	}
	switch status {
	case useraccess.StatusMissing:
		return SubjectAccessDecision{Status: SubjectAccessBlocked, UserID: userID, LoginIdentityID: loginIdentityID}, nil
	case useraccess.StatusBlocked:
		return SubjectAccessDecision{Status: SubjectAccessBlocked, UserID: userID, LoginIdentityID: loginIdentityID}, nil
	case useraccess.StatusInactive:
		return SubjectAccessDecision{Status: SubjectAccessInactive, UserID: userID, LoginIdentityID: loginIdentityID}, nil
	case useraccess.StatusActive:
		// Continue below.
	default:
		return SubjectAccessDecision{}, fmt.Errorf("identity returned unknown user status %q", status)
	}

	return SubjectAccessDecision{
		Status:          SubjectAccessActive,
		UserID:          userID,
		LoginIdentityID: loginIdentityID,
	}, nil
}
