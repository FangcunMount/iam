package account

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/authn/uow"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

func (s *accountApplicationService) GetAccountByID(ctx context.Context, accountID meta.ID) (*AccountResult, error) {
	l := logger.L(ctx)
	var result *AccountResult

	l.Debugw("查询账户",
		"action", logger.ActionRead,
		"resource", "account",
		"account_id", accountID.String(),
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		account, err := tx.Accounts.GetByID(txCtx, accountID)
		if err != nil {
			if isAccountNotFound(err) {
				return accountNotFound(l, accountID)
			}
			l.Errorw("查询账户失败",
				"action", logger.ActionRead,
				"resource", "account",
				"account_id", accountID.String(),
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}
		if account == nil {
			return accountNotFound(l, accountID)
		}
		result = toAccountResult(account)
		return nil
	})
	return result, err
}

func (s *accountApplicationService) FindByExternalRef(
	ctx context.Context,
	accountType domain.AccountType,
	appID domain.AppId,
	externalID domain.ExternalID,
) (*AccountResult, error) {
	var result *AccountResult
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		account, err := tx.Accounts.GetByExternalIDAppId(txCtx, externalID, appID)
		if err != nil {
			if isAccountNotFound(err) {
				return perrors.WithCode(code.ErrCredentialNotFound, "account not found")
			}
			return err
		}
		if account == nil {
			return perrors.WithCode(code.ErrCredentialNotFound, "account not found")
		}
		result = toAccountResult(account)
		return nil
	})
	return result, err
}

func (s *accountApplicationService) FindByUniqueID(ctx context.Context, uniqueID domain.UnionID) (*AccountResult, error) {
	var result *AccountResult
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		account, err := tx.Accounts.GetByUniqueID(txCtx, uniqueID)
		if err != nil {
			if isAccountNotFound(err) {
				return perrors.WithCode(code.ErrCredentialNotFound, "account not found")
			}
			return err
		}
		if account == nil {
			return perrors.WithCode(code.ErrCredentialNotFound, "account not found")
		}
		result = toAccountResult(account)
		return nil
	})
	return result, err
}
