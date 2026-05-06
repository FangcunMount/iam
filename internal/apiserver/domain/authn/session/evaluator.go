package session

import (
	"context"
	"fmt"

	accountdomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/account"
	userdomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type subjectAccessEvaluator struct {
	userRepo    userdomain.Repository
	accountRepo accountdomain.Repository
}

// NewSubjectAccessEvaluator 创建默认的主体访问状态判定器。
func NewSubjectAccessEvaluator(userRepo userdomain.Repository, accountRepo accountdomain.Repository) SubjectAccessEvaluator {
	return &subjectAccessEvaluator{
		userRepo:    userRepo,
		accountRepo: accountRepo,
	}
}

func (e *subjectAccessEvaluator) Evaluate(ctx context.Context, userID meta.ID, accountID meta.ID) (SubjectAccessDecision, error) {
	account, err := e.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return SubjectAccessDecision{}, fmt.Errorf("load account status: %w", err)
	}
	if account == nil {
		return SubjectAccessDecision{Status: SubjectAccessDisabled, UserID: userID, AccountID: accountID}, nil
	}
	if account.IsDisabled() || account.IsArchived() || account.IsDeleted() {
		return SubjectAccessDecision{Status: SubjectAccessDisabled, UserID: userID, AccountID: accountID}, nil
	}

	user, err := e.userRepo.FindByID(ctx, userID)
	if err != nil {
		return SubjectAccessDecision{}, fmt.Errorf("load user status: %w", err)
	}
	if user == nil {
		return SubjectAccessDecision{Status: SubjectAccessBlocked, UserID: userID, AccountID: accountID}, nil
	}
	if user.IsBlocked() {
		return SubjectAccessDecision{Status: SubjectAccessBlocked, UserID: userID, AccountID: accountID}, nil
	}
	if user.IsInactive() {
		return SubjectAccessDecision{Status: SubjectAccessDisabled, UserID: userID, AccountID: accountID}, nil
	}

	return SubjectAccessDecision{
		Status:    SubjectAccessActive,
		UserID:    userID,
		AccountID: accountID,
	}, nil
}
