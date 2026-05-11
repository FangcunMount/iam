package session

import (
	"context"
	"fmt"

	loginidentitydomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	userdomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// SubjectAccessEvaluator 负责汇总 user/login identity 访问状态。
type SubjectAccessEvaluator interface {
	Evaluate(ctx context.Context, userID meta.ID, loginIdentityID meta.ID) (SubjectAccessDecision, error)
}

// subjectAccessEvaluator 负责汇总 user/login identity 访问状态。
type subjectAccessEvaluator struct {
	userRepo          userdomain.Repository
	loginIdentityRepo loginidentitydomain.Repository
}

// NewSubjectAccessEvaluator 创建默认的主体访问状态判定器。
func NewSubjectAccessEvaluator(userRepo userdomain.Repository, loginIdentityRepo loginidentitydomain.Repository) SubjectAccessEvaluator {
	return &subjectAccessEvaluator{
		userRepo:          userRepo,
		loginIdentityRepo: loginIdentityRepo,
	}
}

// Evaluate 评估用户/登录身份的访问状态。
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
	user, err := e.userRepo.FindByID(ctx, userID)
	if err != nil {
		return SubjectAccessDecision{}, fmt.Errorf("load user status: %w", err)
	}
	if user == nil {
		return SubjectAccessDecision{Status: SubjectAccessBlocked, UserID: userID, LoginIdentityID: loginIdentityID}, nil
	}
	if user.IsBlocked() {
		return SubjectAccessDecision{Status: SubjectAccessBlocked, UserID: userID, LoginIdentityID: loginIdentityID}, nil
	}
	if user.IsInactive() {
		return SubjectAccessDecision{Status: SubjectAccessDisabled, UserID: userID, LoginIdentityID: loginIdentityID}, nil
	}

	return SubjectAccessDecision{
		Status:          SubjectAccessActive,
		UserID:          userID,
		LoginIdentityID: loginIdentityID,
	}, nil
}
