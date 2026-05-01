package account

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/uow"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

func (s *accountApplicationService) SetUniqueID(ctx context.Context, accountID meta.ID, uniqueID domain.UnionID) error {
	l := logger.L(ctx)

	l.Debugw("设置账户唯一ID",
		"action", logger.ActionUpdate,
		"resource", "account",
		"account_id", accountID.String(),
		"unique_id", string(uniqueID),
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		editor := domain.NewEditor(tx.Accounts)
		_, err := editor.SetUniqueID(txCtx, accountID, uniqueID)
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

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		editor := domain.NewEditor(tx.Accounts)
		_, err := editor.UpdateProfile(txCtx, accountID, profile)
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
	return s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		editor := domain.NewEditor(tx.Accounts)
		_, err := editor.UpdateMeta(txCtx, accountID, meta)
		return err
	})
}
