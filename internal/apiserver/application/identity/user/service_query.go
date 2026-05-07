package user

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ===========================================
// ==== Directory 实现 =====
// ===========================================

// directory 用户查询用例实现
type directory struct {
	uow uow.UnitOfWork
}

// NewDirectory 创建用户查询用例
func NewDirectory(uow uow.UnitOfWork) Directory {
	return &directory{uow: uow}
}

// GetByID 根据 ID 查询用户
func (s *directory) GetByID(ctx context.Context, userID meta.ID) (*UserResult, error) {
	l := logger.L(ctx)
	l.Debugw("查询用户信息",
		"action", logger.ActionRead,
		"resource", logger.ResourceUser,
		"user_id", userID,
	)

	var result *UserResult

	// 查询操作也通过 UoW，但不需要写操作，可以直接使用只读事务
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		user, err := tx.Users.FindByID(txCtx, userID)
		if err != nil {
			l.Warnw("用户不存在",
				"action", logger.ActionRead,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			if isUserNotFound(err) {
				return perrors.WithCode(code.ErrUserNotFound, "user(%s) not found", userID)
			}
			return err
		}

		result = toUserResult(user)
		return nil
	})

	if err == nil {
		l.Debugw("查询用户成功",
			"action", logger.ActionRead,
			"resource", logger.ResourceUser,
			"user_id", result.ID,
			"result", logger.ResultSuccess,
		)
	}

	return result, err
}

// BatchGetByID 根据 ID 集合批量查询用户。
func (s *directory) BatchGetByID(ctx context.Context, userIDs []meta.ID) (map[string]*UserResult, error) {
	results := map[string]*UserResult{}
	if len(userIDs) == 0 {
		return results, nil
	}

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		ids := userIDsForBatch(userIDs)
		if len(ids) == 0 {
			return nil
		}
		usersByID, err := tx.Users.FindByIDs(txCtx, ids)
		if err != nil {
			return err
		}
		for _, u := range usersByID {
			if u == nil {
				continue
			}
			results[u.ID.String()] = toUserResult(u)
		}
		return nil
	})

	return results, err
}

func userIDsForBatch(userIDs []meta.ID) []meta.ID {
	ids := make([]meta.ID, 0, len(userIDs))
	seen := make(map[meta.ID]struct{}, len(userIDs))
	for _, id := range userIDs {
		if id.IsZero() {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

// GetByPhone 根据手机号查询用户
func (s *directory) GetByPhone(ctx context.Context, phone string) (*UserResult, error) {
	var result *UserResult

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		phoneObj, err := optionalPhone(phone)
		if err != nil {
			return err
		}

		user, err := tx.Users.FindByPhone(txCtx, phoneObj)
		if err != nil {
			return err
		}
		if user == nil {
			return perrors.WithCode(code.ErrUserNotFound, "user with phone(%s) not found", phoneObj.String())
		}

		result = toUserResult(user)
		return nil
	})

	return result, err
}
