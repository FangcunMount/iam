package onboarding

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type accountCreation struct {
	Account        *domain.Account
	CreationParams *domain.CreationParams
	IsNewAccount   bool
}

// AccountProvisioner 负责开通流程中的账户创建输入组装和持久化。
type AccountProvisioner struct{}

func newAccountProvisioner() *AccountProvisioner {
	return &AccountProvisioner{}
}

func (c *AccountProvisioner) Provision(ctx context.Context, repo domain.Repository, req OnboardingRequest, userID meta.ID) (*accountCreation, error) {
	domainInput, err := c.toDomainInput(req, userID)
	if err != nil {
		return nil, err
	}

	accountCreator := domain.NewAccountCreator(repo)
	account, creationParams, err := accountCreator.CreateAccount(ctx, domainInput)
	if err != nil {
		return nil, err
	}

	isNewAccount := false
	if account.ID.IsZero() {
		if err := repo.Create(ctx, account); err != nil {
			return nil, perrors.WithCode(code.ErrDatabase, "failed to save account: %v", err)
		}
		isNewAccount = true
	}

	return &accountCreation{
		Account:        account,
		CreationParams: creationParams,
		IsNewAccount:   isNewAccount,
	}, nil
}

func (c *AccountProvisioner) toDomainInput(req OnboardingRequest, userID meta.ID) (domain.CreationInput, error) {
	input := domain.CreationInput{
		UserID:         userID,
		Phone:          req.Phone,
		Email:          req.Email,
		OperaLoginID:   strings.TrimSpace(req.OperaLoginID),
		ScopedTenantID: req.ScopedTenantID,
		AccountType:    req.AccountType,
		WechatAppID:    req.WechatAppID,
		WechatOpenID:   req.WechatOpenID,
		WechatUnionID:  req.WechatUnionID,
		WecomCorpID:    req.WecomCorpID,
		WecomUserID:    req.WecomUserID,
		Profile:        req.Profile,
		Meta:           req.Meta,
		ParamsJSON:     req.ParamsJSON,
	}

	return input, nil
}
