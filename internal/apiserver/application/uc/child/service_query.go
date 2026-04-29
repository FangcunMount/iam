package child

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/input"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
)

// ============================================
// ==== ChildQueryApplicationService 实现 =====
// ============================================

// childQueryApplicationService 儿童查询应用服务实现
type childQueryApplicationService struct {
	uow uow.UnitOfWork
}

// NewChildQueryApplicationService 创建儿童查询应用服务
func NewChildQueryApplicationService(uow uow.UnitOfWork) ChildQueryApplicationService {
	return &childQueryApplicationService{uow: uow}
}

// GetByID 根据 ID 查询儿童
func (s *childQueryApplicationService) GetByID(ctx context.Context, childID string) (*ChildResult, error) {
	l := logger.L(ctx)
	var result *ChildResult

	l.Debugw("开始查询儿童档案",
		"action", logger.ActionRead,
		"resource", logger.ResourceChild,
		"resource_id", childID,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		childIDObj, err := parseChildID(childID)
		if err != nil {
			l.Warnw("儿童ID解析失败",
				"action", logger.ActionRead,
				"resource", logger.ResourceChild,
				"resource_id", childID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		child, err := tx.Children.FindByID(txCtx, childIDObj)
		if err != nil {
			l.Warnw("查询儿童档案失败",
				"action", logger.ActionRead,
				"resource", logger.ResourceChild,
				"resource_id", childID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		result = toChildResult(child)
		return nil
	})

	if err == nil {
		l.Debugw("查询儿童档案成功",
			"action", logger.ActionRead,
			"resource", logger.ResourceChild,
			"resource_id", childID,
			"result", logger.ResultSuccess,
		)
	}

	return result, err
}

// GetByIDCard 根据身份证查询儿童
func (s *childQueryApplicationService) GetByIDCard(ctx context.Context, idCard string) (*ChildResult, error) {
	l := logger.L(ctx)
	var result *ChildResult

	l.Debugw("开始根据身份证查询儿童档案",
		"action", logger.ActionRead,
		"resource", logger.ResourceChild,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		idCardObj, err := input.ParseIDCard("", idCard)
		if err != nil {
			l.Warnw("身份证格式验证失败",
				"action", logger.ActionRead,
				"resource", logger.ResourceChild,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		child, err := tx.Children.FindByIDCard(txCtx, idCardObj)
		if err != nil {
			l.Warnw("根据身份证查询儿童档案失败",
				"action", logger.ActionRead,
				"resource", logger.ResourceChild,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		result = toChildResult(child)
		return nil
	})

	if err == nil && result != nil {
		l.Debugw("根据身份证查询儿童档案成功",
			"action", logger.ActionRead,
			"resource", logger.ResourceChild,
			"resource_id", result.ID,
			"result", logger.ResultSuccess,
		)
	}

	return result, err
}

// FindSimilar 查找相似儿童（姓名、性别、生日）
func (s *childQueryApplicationService) FindSimilar(ctx context.Context, name string, gender uint8, birthday string) ([]*ChildResult, error) {
	l := logger.L(ctx)
	var results []*ChildResult

	l.Debugw("开始查找相似儿童档案",
		"action", logger.ActionList,
		"resource", logger.ResourceChild,
		"name", name,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		genderObj := input.ParseGender(gender)
		birthdayObj := input.ParseBirthday(birthday)

		children, err := tx.Children.FindSimilar(txCtx, name, genderObj, birthdayObj)
		if err != nil {
			l.Warnw("查找相似儿童档案失败",
				"action", logger.ActionList,
				"resource", logger.ResourceChild,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		results = toChildResults(children)
		return nil
	})

	if err == nil {
		l.Debugw("查找相似儿童档案成功",
			"action", logger.ActionList,
			"resource", logger.ResourceChild,
			"count", len(results),
			"result", logger.ResultSuccess,
		)
	}

	return results, err
}
