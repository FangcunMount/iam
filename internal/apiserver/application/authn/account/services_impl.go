package account

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/authn/uow"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	sessiondomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ============= AccountApplicationService 实现 =============

// accountApplicationService 账户应用服务实现
type accountApplicationService struct {
	uow            uow.UnitOfWork
	sessionManager sessiondomain.Manager
}

// accountApplicationService 实现 AccountApplicationService 接口
var _ AccountApplicationService = (*accountApplicationService)(nil)

// NewAccountApplicationService 创建账户应用服务
func NewAccountApplicationService(uow uow.UnitOfWork, sessionManager sessiondomain.Manager) AccountApplicationService {
	return &accountApplicationService{uow: uow, sessionManager: sessionManager}
}

// GetAccountByID 根据ID获取账户
func (s *accountApplicationService) GetAccountByID(ctx context.Context, accountID meta.ID) (*AccountResult, error) {
	l := logger.L(ctx)
	var result *AccountResult

	l.Debugw("查询账户",
		"action", logger.ActionRead,
		"resource", "account",
		"account_id", accountID.String(),
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		account, err := tx.Accounts.GetByID(ctx, accountID)
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
	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		account, err := tx.Accounts.GetByExternalIDAppId(ctx, externalID, appID)
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
	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		account, err := tx.Accounts.GetByUniqueID(ctx, uniqueID)
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

func (s *accountApplicationService) SetUniqueID(ctx context.Context, accountID meta.ID, uniqueID domain.UnionID) error {
	l := logger.L(ctx)

	l.Debugw("设置账户唯一ID",
		"action", logger.ActionUpdate,
		"resource", "account",
		"account_id", accountID.String(),
		"unique_id", string(uniqueID),
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		editor := domain.NewEditor(tx.Accounts)
		_, err := editor.SetUniqueID(ctx, accountID, uniqueID)
		if err != nil {
			l.Errorw("设置账户唯一ID失败",
				"action", logger.ActionUpdate,
				"resource", "account",
				"account_id", accountID.String(),
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
		}
		return err
	})

	if err == nil {
		l.Debugw("账户唯一ID设置成功",
			"action", logger.ActionUpdate,
			"resource", "account",
			"account_id", accountID.String(),
			"result", logger.ResultSuccess,
		)
	}

	return err
}

func (s *accountApplicationService) UpdateProfile(ctx context.Context, accountID meta.ID, profile map[string]string) error {
	l := logger.L(ctx)

	l.Debugw("更新账户资料",
		"action", logger.ActionUpdate,
		"resource", "account",
		"account_id", accountID.String(),
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		editor := domain.NewEditor(tx.Accounts)
		_, err := editor.UpdateProfile(ctx, accountID, profile)
		if err != nil {
			l.Errorw("更新账户资料失败",
				"action", logger.ActionUpdate,
				"resource", "account",
				"account_id", accountID.String(),
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
		}
		return err
	})

	if err == nil {
		l.Debugw("账户资料更新成功",
			"action", logger.ActionUpdate,
			"resource", "account",
			"account_id", accountID.String(),
			"result", logger.ResultSuccess,
		)
	}

	return err
}

func (s *accountApplicationService) UpdateMeta(ctx context.Context, accountID meta.ID, meta map[string]string) error {
	return s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		editor := domain.NewEditor(tx.Accounts)
		_, err := editor.UpdateMeta(ctx, accountID, meta)
		return err
	})
}

func (s *accountApplicationService) EnableAccount(ctx context.Context, accountID meta.ID) error {
	return s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 使用新的 StatusManager 接口
		statusManager := domain.NewStatusManager(tx.Accounts)
		account, err := statusManager.Activate(ctx, accountID)
		if err != nil {
			return err
		}

		// 持久化状态变更
		return tx.Accounts.UpdateStatus(ctx, account.ID, account.Status)
	})
}

func (s *accountApplicationService) DisableAccount(ctx context.Context, accountID meta.ID) error {
	if err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 使用新的 StatusManager 接口
		statusManager := domain.NewStatusManager(tx.Accounts)
		account, err := statusManager.Disable(ctx, accountID)
		if err != nil {
			return err
		}

		// 持久化状态变更
		return tx.Accounts.UpdateStatus(ctx, account.ID, account.Status)
	}); err != nil {
		return err
	}
	if s.sessionManager == nil {
		return nil
	}
	return s.sessionManager.RevokeByAccount(ctx, accountID, "account_disabled", accountID.String())
}

func (s *accountApplicationService) ArchiveAccount(ctx context.Context, accountID meta.ID) error {
	return s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 使用新的 StatusManager 接口
		statusManager := domain.NewStatusManager(tx.Accounts)
		account, err := statusManager.Archive(ctx, accountID)
		if err != nil {
			return err
		}

		// 持久化状态变更
		return tx.Accounts.UpdateStatus(ctx, account.ID, account.Status)
	})
}

func (s *accountApplicationService) DeleteAccount(ctx context.Context, accountID meta.ID) error {
	return s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 使用新的 StatusManager 接口
		statusManager := domain.NewStatusManager(tx.Accounts)
		account, err := statusManager.Delete(ctx, accountID)
		if err != nil {
			return err
		}

		// 持久化状态变更
		return tx.Accounts.UpdateStatus(ctx, account.ID, account.Status)
	})
}

// ============= Helper Functions =============

func toAccountResult(account *domain.Account) *AccountResult {
	return &AccountResult{
		AccountID:  account.ID,
		UserID:     account.UserID,
		Type:       account.Type,
		AppID:      account.AppID,
		ExternalID: account.ExternalID,
		UniqueID:   account.UniqueID,
		Profile:    account.Profile,
		Meta:       account.Meta,
		Status:     account.Status,
	}
}

func isAccountNotFound(err error) bool {
	return perrors.IsCode(err, code.ErrCredentialNotFound) ||
		perrors.IsCode(err, code.ErrNotFoundAccount)
}

func accountNotFound(l *logger.RequestLogger, accountID meta.ID) error {
	l.Warnw("账户不存在",
		"action", logger.ActionRead,
		"resource", "account",
		"account_id", accountID.String(),
		"result", logger.ResultFailed,
	)
	return perrors.WithCode(code.ErrCredentialNotFound, "account not found")
}
