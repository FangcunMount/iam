package onboarding

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	accountDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// AccountEnsureStatus 表示账号确保阶段的结果。
type AccountEnsureStatus string

const (
	AccountCreated AccountEnsureStatus = "created"
	AccountReused  AccountEnsureStatus = "reused"
)

// AccountEnsureResult 是账号确保阶段的显式结果。
type AccountEnsureResult struct {
	Account        *accountDomain.Account
	CreationParams *accountDomain.CreationParams
	Status         AccountEnsureStatus
}

func (r AccountEnsureResult) IsNewAccount() bool {
	return r.Status == AccountCreated
}

type accountEnsurer struct{}

func newAccountEnsurer() *accountEnsurer {
	return &accountEnsurer{}
}

func (e *accountEnsurer) Ensure(
	ctx context.Context,
	repo accountDomain.Repository,
	req *NormalizedOnboardingRequest,
	userID meta.ID,
) (*AccountEnsureResult, error) {
	domainInput, err := e.toDomainInput(req, userID)
	if err != nil {
		return nil, err
	}

	accountCreator := accountDomain.NewAccountCreator(repo)
	account, creationParams, err := accountCreator.CreateAccount(ctx, domainInput)
	if err != nil {
		return nil, err
	}

	status, err := e.persistIfCreated(ctx, repo, account)
	if err != nil {
		return nil, err
	}

	return &AccountEnsureResult{
		Account:        account,
		CreationParams: creationParams,
		Status:         status,
	}, nil
}

func (e *accountEnsurer) persistIfCreated(ctx context.Context, repo accountDomain.Repository, account *accountDomain.Account) (AccountEnsureStatus, error) {
	if !account.ID.IsZero() {
		return AccountReused, nil
	}
	if err := repo.Create(ctx, account); err != nil {
		return "", perrors.WithCode(code.ErrDatabase, "failed to save account: %v", err)
	}
	return AccountCreated, nil
}

func (e *accountEnsurer) toDomainInput(req *NormalizedOnboardingRequest, userID meta.ID) (accountDomain.CreationInput, error) {
	return accountDomain.CreationInput{
		UserID:         userID,
		Phone:          req.Phone,
		Email:          req.Email,
		OperaLoginID:   strings.TrimSpace(req.OperaLoginID),
		ScopedTenantID: req.ScopedTenantID,
		AccountType:    req.Plan.AccountType,
		WechatAppID:    req.WechatAppID,
		WechatOpenID:   req.WechatOpenID,
		WechatUnionID:  req.WechatUnionID,
		WecomCorpID:    req.WecomCorpID,
		WecomUserID:    req.WecomUserID,
		Profile:        req.Profile,
		Meta:           req.Meta,
		ParamsJSON:     req.ParamsJSON,
	}, nil
}
