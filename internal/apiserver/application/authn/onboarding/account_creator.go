package onboarding

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

type accountCreation struct {
	Account        *domain.Account
	CreationParams *domain.CreationParams
	IsNewAccount   bool
}

// AccountCreator 负责注册流程中的账户创建输入组装和持久化。
type AccountCreator struct {
	idp authentication.IdentityProvider
}

func newAccountCreator(idp authentication.IdentityProvider) *AccountCreator {
	return &AccountCreator{idp: idp}
}

func (c *AccountCreator) Create(ctx context.Context, repo domain.Repository, req OnboardingRequest, userID meta.ID) (*accountCreation, error) {
	domainInput, err := c.toDomainInput(req, userID)
	if err != nil {
		return nil, err
	}

	accountCreator := domain.NewAccountCreator(repo, c.idp)
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

func (c *AccountCreator) toDomainInput(req OnboardingRequest, userID meta.ID) (domain.CreationInput, error) {
	input := domain.CreationInput{
		UserID:         userID,
		Phone:          req.Phone,
		Email:          req.Email,
		OperaLoginID:   strings.TrimSpace(req.OperaLoginID),
		ScopedTenantID: req.ScopedTenantID,
		AccountType:    req.AccountType,
		WechatAppID:    req.WechatAppID,
		WechatJsCode:   req.WechatJsCode,
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
