package guardianship

import (
	"context"
	"fmt"
	"time"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	childdomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/child"
	gsshipdomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/guardianship"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ============= 应用服务实现 =============

// =============================================
// ==== GuardianshipApplicationService 实现 =====
// =============================================
// guardianshipApplicationService 监护关系应用服务实现
type guardianshipApplicationService struct {
	uow uow.UnitOfWork
}

// NewGuardianshipApplicationService 创建监护关系应用服务
func NewGuardianshipApplicationService(uow uow.UnitOfWork) GuardianshipApplicationService {
	return &guardianshipApplicationService{uow: uow}
}

// AddGuardian 添加监护人
func (s *guardianshipApplicationService) AddGuardian(ctx context.Context, dto AddGuardianDTO) error {
	l := logger.L(ctx)
	l.Debugw("添加监护人",
		"action", logger.ActionCreate,
		"resource", "guardianship",
		"user_id", dto.UserID,
		"child_id", dto.ChildID,
		"relation", dto.Relation,
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建领域服务
		managerService := gsshipdomain.NewManagerService(tx.Guardianships, tx.Children, tx.Users)

		// 转换 ID
		userID, err := parseUserID(dto.UserID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionCreate,
				"resource", "guardianship",
				"error", err.Error(),
			)
			return err
		}
		childID, err := parseChildID(dto.ChildID)
		if err != nil {
			l.Warnw("儿童ID格式错误",
				"action", logger.ActionCreate,
				"resource", "guardianship",
				"error", err.Error(),
			)
			return err
		}

		// 转换关系
		relation := ParseRelation(dto.Relation)

		// 调用领域服务添加监护人
		guardianship, err := managerService.AddGuardian(ctx, userID, childID, relation)
		if err != nil {
			l.Errorw("添加监护人失败",
				"action", logger.ActionCreate,
				"resource", "guardianship",
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化监护关系
		return tx.Guardianships.Create(ctx, guardianship)
	})

	if err == nil {
		l.Debugw("添加监护人成功",
			"action", logger.ActionCreate,
			"resource", "guardianship",
			"user_id", dto.UserID,
			"child_id", dto.ChildID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// RemoveGuardian 移除监护人
func (s *guardianshipApplicationService) RemoveGuardian(ctx context.Context, dto RemoveGuardianDTO) error {
	l := logger.L(ctx)
	l.Debugw("移除监护人",
		"action", logger.ActionDelete,
		"resource", "guardianship",
		"user_id", dto.UserID,
		"child_id", dto.ChildID,
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建领域服务
		managerService := gsshipdomain.NewManagerService(tx.Guardianships, tx.Children, tx.Users)

		// 转换 ID
		userID, err := parseUserID(dto.UserID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionDelete,
				"resource", "guardianship",
				"error", err.Error(),
			)
			return err
		}
		childID, err := parseChildID(dto.ChildID)
		if err != nil {
			l.Warnw("儿童ID格式错误",
				"action", logger.ActionDelete,
				"resource", "guardianship",
				"error", err.Error(),
			)
			return err
		}

		// 调用领域服务移除监护人
		guardianship, err := managerService.RemoveGuardian(ctx, userID, childID)
		if err != nil {
			l.Errorw("移除监护人失败",
				"action", logger.ActionDelete,
				"resource", "guardianship",
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Guardianships.Update(ctx, guardianship)
	})

	if err == nil {
		l.Debugw("移除监护人成功",
			"action", logger.ActionDelete,
			"resource", "guardianship",
			"user_id", dto.UserID,
			"child_id", dto.ChildID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// ===========================================
// === GuardianshipQueryApplicationService 实现 ===
// ===========================================

// GetByUserIDAndChildID 查询监护关系
func (s *guardianshipApplicationService) GetByUserIDAndChildID(ctx context.Context, userID string, childID string) (*GuardianshipResult, error) {
	l := logger.L(ctx)
	l.Debugw("查询监护关系",
		"action", logger.ActionRead,
		"resource", "guardianship",
		"user_id", userID,
		"child_id", childID,
	)

	var result *GuardianshipResult

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建领域服务

		// 转换 ID
		uid, err := parseUserID(userID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionRead,
				"resource", "guardianship",
				"error", err.Error(),
			)
			return err
		}
		cid, err := parseChildID(childID)
		if err != nil {
			l.Warnw("儿童ID格式错误",
				"action", logger.ActionRead,
				"resource", "guardianship",
				"error", err.Error(),
			)
			return err
		}

		// 调用领域服务查询
		guardianship, err := tx.Guardianships.FindByUserIDAndChildID(ctx, uid, cid)
		if err != nil {
			l.Warnw("监护关系不存在",
				"action", logger.ActionRead,
				"resource", "guardianship",
				"error", err.Error(),
			)
			return err
		}

		// 查询儿童信息
		child, err := tx.Children.FindByID(ctx, guardianship.Child)
		if err != nil {
			l.Errorw("查询儿童信息失败",
				"action", logger.ActionRead,
				"resource", "guardianship",
				"error", err.Error(),
			)
			return err
		}

		// 转换为 DTO
		result = toGuardianshipResult(guardianship, child)
		return nil
	})

	if err == nil {
		l.Debugw("查询监护关系成功",
			"action", logger.ActionRead,
			"resource", "guardianship",
			"result", logger.ResultSuccess,
		)
	}

	return result, err
}

// ListChildrenByUserID 列出用户监护的所有儿童
func (s *guardianshipApplicationService) ListChildrenByUserID(ctx context.Context, userID string) ([]*GuardianshipResult, error) {
	l := logger.L(ctx)
	l.Debugw("查询用户监护的儿童列表",
		"action", logger.ActionRead,
		"resource", "guardianship",
		"user_id", userID,
	)

	var results []*GuardianshipResult

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建领域服务

		// 转换 ID
		uid, err := parseUserID(userID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionRead,
				"resource", "guardianship",
				"error", err.Error(),
			)
			return err
		}

		// 调用领域服务查询
		guardianships, err := tx.Guardianships.FindByUserID(ctx, uid)
		if err != nil {
			l.Errorw("查询监护关系失败",
				"action", logger.ActionRead,
				"resource", "guardianship",
				"error", err.Error(),
			)
			return err
		}

		// 查询儿童信息并转换为 DTO
		for _, g := range guardianships {
			child, err := tx.Children.FindByID(ctx, g.Child)
			if err != nil {
				l.Warnw("查询儿童信息失败，跳过",
					"action", logger.ActionRead,
					"resource", "guardianship",
					"child_id", g.Child.String(),
					"error", err.Error(),
				)
				continue
			}
			results = append(results, toGuardianshipResult(g, child))
		}

		return nil
	})

	return results, err
}

// ListGuardiansByChildID 列出儿童的所有监护人
func (s *guardianshipApplicationService) ListGuardiansByChildID(ctx context.Context, childID string) ([]*GuardianshipResult, error) {
	var results []*GuardianshipResult

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建领域服务

		// 转换 ID
		cid, err := parseChildID(childID)
		if err != nil {
			return err
		}

		// 调用领域服务查询
		guardianships, err := tx.Guardianships.FindByChildID(ctx, cid)
		if err != nil {
			return err
		}

		// 查询儿童信息并转换为 DTO
		child, err := tx.Children.FindByID(ctx, cid)
		if err != nil {
			return err
		}

		for _, g := range guardianships {
			results = append(results, toGuardianshipResult(g, child))
		}

		return nil
	})

	return results, err
}

// ==================================================
// ==== GuardianshipQueryApplicationService 实现 =====
// ==================================================

// guardianshipQueryApplicationService 监护关系查询应用服务实现
type guardianshipQueryApplicationService struct {
	uow uow.UnitOfWork
}

// NewGuardianshipQueryApplicationService 创建监护关系查询应用服务
func NewGuardianshipQueryApplicationService(uow uow.UnitOfWork) GuardianshipQueryApplicationService {
	return &guardianshipQueryApplicationService{uow: uow}
}

// IsGuardian 检查是否为监护人
func (s *guardianshipQueryApplicationService) IsGuardian(ctx context.Context, userID string, childID string) (bool, error) {
	var isGuardian bool

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {

		userIDObj, err := parseUserID(userID)
		if err != nil {
			return err
		}

		childIDObj, err := parseChildID(childID)
		if err != nil {
			return err
		}

		isGuardian, err = tx.Guardianships.IsGuardian(ctx, userIDObj, childIDObj)
		return err
	})

	return isGuardian, err
}

// GetByUserIDAndChildID 查询监护关系
func (s *guardianshipQueryApplicationService) GetByUserIDAndChildID(ctx context.Context, userID string, childID string) (*GuardianshipResult, error) {
	return s.getByUserIDAndChildID(ctx, userID, childID, false)
}

// GetByUserIDAndChildIDIncludingRevoked 查询监护关系（包含已撤销）
func (s *guardianshipQueryApplicationService) GetByUserIDAndChildIDIncludingRevoked(ctx context.Context, userID string, childID string) (*GuardianshipResult, error) {
	return s.getByUserIDAndChildID(ctx, userID, childID, true)
}

func (s *guardianshipQueryApplicationService) getByUserIDAndChildID(ctx context.Context, userID string, childID string, includeRevoked bool) (*GuardianshipResult, error) {
	var result *GuardianshipResult

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {

		userIDObj, err := parseUserID(userID)
		if err != nil {
			return err
		}

		childIDObj, err := parseChildID(childID)
		if err != nil {
			return err
		}

		var guardianship *gsshipdomain.Guardianship
		if includeRevoked {
			guardianship, err = tx.Guardianships.FindByUserIDAndChildIDIncludingRevoked(ctx, userIDObj, childIDObj)
		} else {
			guardianship, err = tx.Guardianships.FindByUserIDAndChildID(ctx, userIDObj, childIDObj)
		}
		if err != nil {
			return err
		}

		// 查询儿童信息
		child, err := tx.Children.FindByID(ctx, guardianship.Child)
		if err != nil {
			return err
		}

		result = toGuardianshipResult(guardianship, child)
		return nil
	})

	return result, err
}

// ListChildrenByUserID 列出用户监护的所有儿童
func (s *guardianshipQueryApplicationService) ListChildrenByUserID(ctx context.Context, userID string) ([]*GuardianshipResult, error) {
	return s.listChildrenByUserID(ctx, userID, false)
}

// ListChildrenByUserIDIncludingRevoked 列出用户监护的所有儿童（包含已撤销）
func (s *guardianshipQueryApplicationService) ListChildrenByUserIDIncludingRevoked(ctx context.Context, userID string) ([]*GuardianshipResult, error) {
	return s.listChildrenByUserID(ctx, userID, true)
}

func (s *guardianshipQueryApplicationService) listChildrenByUserID(ctx context.Context, userID string, includeRevoked bool) ([]*GuardianshipResult, error) {
	var results []*GuardianshipResult

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {

		userIDObj, err := parseUserID(userID)
		if err != nil {
			return err
		}

		var guardianships []*gsshipdomain.Guardianship
		if includeRevoked {
			guardianships, err = tx.Guardianships.FindByUserIDIncludingRevoked(ctx, userIDObj)
		} else {
			guardianships, err = tx.Guardianships.FindByUserID(ctx, userIDObj)
		}
		if err != nil {
			return err
		}

		// 遍历查询每个儿童信息
		results = make([]*GuardianshipResult, 0, len(guardianships))
		for _, g := range guardianships {
			if g == nil {
				continue
			}
			child, err := tx.Children.FindByID(ctx, g.Child)
			if err != nil {
				return err
			}
			results = append(results, toGuardianshipResult(g, child))
		}

		return nil
	})

	return results, err
}

// ListGuardiansByChildID 列出儿童的所有监护人
func (s *guardianshipQueryApplicationService) ListGuardiansByChildID(ctx context.Context, childID string) ([]*GuardianshipResult, error) {
	return s.listGuardiansByChildID(ctx, childID, false)
}

// ListGuardiansByChildIDIncludingRevoked 列出儿童的所有监护人（包含已撤销）
func (s *guardianshipQueryApplicationService) ListGuardiansByChildIDIncludingRevoked(ctx context.Context, childID string) ([]*GuardianshipResult, error) {
	return s.listGuardiansByChildID(ctx, childID, true)
}

func (s *guardianshipQueryApplicationService) listGuardiansByChildID(ctx context.Context, childID string, includeRevoked bool) ([]*GuardianshipResult, error) {
	var results []*GuardianshipResult

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {

		childIDObj, err := parseChildID(childID)
		if err != nil {
			return err
		}

		var guardianships []*gsshipdomain.Guardianship
		if includeRevoked {
			guardianships, err = tx.Guardianships.FindByChildIDIncludingRevoked(ctx, childIDObj)
		} else {
			guardianships, err = tx.Guardianships.FindByChildID(ctx, childIDObj)
		}
		if err != nil {
			return err
		}

		// 查询儿童信息（所有监护关系共享同一个儿童）
		child, err := tx.Children.FindByID(ctx, childIDObj)
		if err != nil {
			return err
		}

		results = make([]*GuardianshipResult, 0, len(guardianships))
		for _, g := range guardianships {
			if g == nil {
				continue
			}
			results = append(results, toGuardianshipResult(g, child))
		}

		return nil
	})

	return results, err
}

// ============= DTO 转换辅助函数 =============

// parseUserID 解析用户ID字符串
func parseUserID(userID string) (meta.ID, error) {
	var id uint64
	_, err := fmt.Sscanf(userID, "%d", &id)
	if err != nil {
		return meta.FromUint64(0), err
	}

	return meta.FromUint64(id), nil
}

// parseChildID 解析儿童ID字符串
func parseChildID(childID string) (meta.ID, error) {
	var id uint64
	_, err := fmt.Sscanf(childID, "%d", &id)
	if err != nil {
		return meta.FromUint64(0), err
	}
	return meta.FromUint64(id), nil
}

// toGuardianshipResult 将领域实体转换为 DTO
func toGuardianshipResult(guardianship *gsshipdomain.Guardianship, child *childdomain.Child) *GuardianshipResult {
	if guardianship == nil {
		return nil
	}

	result := &GuardianshipResult{
		ID:            guardianship.ID.Uint64(),
		UserID:        guardianship.User.String(),
		ChildID:       guardianship.Child.String(),
		Relation:      string(guardianship.Rel), // Relation 是 string 类型
		EstablishedAt: guardianship.EstablishedAt.Format(time.RFC3339),
	}
	if guardianship.RevokedAt != nil && !guardianship.RevokedAt.IsZero() {
		result.RevokedAt = guardianship.RevokedAt.Format(time.RFC3339)
	}

	// 添加儿童信息
	if child != nil {
		result.ChildName = child.Name
		result.ChildGender = child.Gender.Value()
		result.ChildBirthday = child.Birthday.String()
	}

	return result
}
