package account

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/uow"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

func (s *accountApplicationService) EnableAccount(ctx context.Context, accountID meta.ID) error {
	return s.changeStatus(ctx, accountID, func(manager domain.StatusManager, txCtx context.Context) (*domain.Account, error) {
		return manager.Activate(txCtx, accountID)
	})
}

func (s *accountApplicationService) DisableAccount(ctx context.Context, accountID meta.ID) error {
	if err := s.changeStatus(ctx, accountID, func(manager domain.StatusManager, txCtx context.Context) (*domain.Account, error) {
		return manager.Disable(txCtx, accountID)
	}); err != nil {
		return err
	}
	if s.sessionManager == nil {
		return nil
	}
	return s.sessionManager.RevokeByAccount(ctx, accountID, "account_disabled", accountID.String())
}

func (s *accountApplicationService) ArchiveAccount(ctx context.Context, accountID meta.ID) error {
	return s.changeStatus(ctx, accountID, func(manager domain.StatusManager, txCtx context.Context) (*domain.Account, error) {
		return manager.Archive(txCtx, accountID)
	})
}

func (s *accountApplicationService) DeleteAccount(ctx context.Context, accountID meta.ID) error {
	return s.changeStatus(ctx, accountID, func(manager domain.StatusManager, txCtx context.Context) (*domain.Account, error) {
		return manager.Delete(txCtx, accountID)
	})
}

func (s *accountApplicationService) changeStatus(
	ctx context.Context,
	accountID meta.ID,
	change func(domain.StatusManager, context.Context) (*domain.Account, error),
) error {
	return s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		statusManager := domain.NewStatusManager(tx.Accounts)
		account, err := change(statusManager, txCtx)
		if err != nil {
			return err
		}
		return tx.Accounts.UpdateStatus(txCtx, account.ID, account.Status)
	})
}
