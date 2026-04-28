package register

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	idpPort "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
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
	idp              authentication.IdentityProvider
	wechatAppQuerier idpPort.Repository
	secretVault      idpPort.SecretVault
}

func newAccountCreator(
	idp authentication.IdentityProvider,
	wechatAppQuerier idpPort.Repository,
	secretVault idpPort.SecretVault,
) *AccountCreator {
	return &AccountCreator{
		idp:              idp,
		wechatAppQuerier: wechatAppQuerier,
		secretVault:      secretVault,
	}
}

func (c *AccountCreator) Create(ctx context.Context, repo domain.Repository, req RegisterRequest, userID meta.ID) (*accountCreation, error) {
	domainInput, err := c.toDomainInput(ctx, req, userID)
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

func (c *AccountCreator) toDomainInput(ctx context.Context, req RegisterRequest, userID meta.ID) (domain.CreationInput, error) {
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

	if req.AccountType == domain.TypeWcMinip && req.WechatAppID != nil && req.WechatJsCode != nil {
		if c.wechatAppQuerier == nil || c.secretVault == nil {
			return domain.CreationInput{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app configuration service not available")
		}

		wechatApp, err := c.wechatAppQuerier.GetByAppID(ctx, *req.WechatAppID)
		if err != nil {
			return domain.CreationInput{}, perrors.WithCode(code.ErrInvalidArgument, "failed to query wechat app: %v", err)
		}
		if wechatApp == nil {
			return domain.CreationInput{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app not found: %s", *req.WechatAppID)
		}
		if !wechatApp.IsEnabled() {
			return domain.CreationInput{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app is disabled: %s", *req.WechatAppID)
		}
		if wechatApp.Cred == nil || wechatApp.Cred.Auth == nil {
			return domain.CreationInput{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app credentials not found")
		}

		appSecretPlain, err := c.secretVault.Decrypt(ctx, wechatApp.Cred.Auth.AppSecretCipher)
		if err != nil {
			return domain.CreationInput{}, perrors.WithCode(code.ErrInvalidArgument, "failed to decrypt app secret: %v", err)
		}

		appSecret := string(appSecretPlain)
		input.WechatAppSecret = &appSecret
	}

	return input, nil
}
