package guardianship

import (
	"context"

	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/guardianship"
)

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

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		userIDObj, err := parseUserID(userID)
		if err != nil {
			return err
		}

		childIDObj, err := parseChildID(childID)
		if err != nil {
			return err
		}

		isGuardian, err = tx.Guardianships.IsGuardian(txCtx, userIDObj, childIDObj)
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

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		userIDObj, err := parseUserID(userID)
		if err != nil {
			return err
		}

		childIDObj, err := parseChildID(childID)
		if err != nil {
			return err
		}

		var guardianship *domain.Guardianship
		if includeRevoked {
			guardianship, err = tx.Guardianships.FindByUserIDAndChildIDIncludingRevoked(txCtx, userIDObj, childIDObj)
		} else {
			guardianship, err = tx.Guardianships.FindByUserIDAndChildID(txCtx, userIDObj, childIDObj)
		}
		if err != nil {
			return err
		}

		// 查询儿童信息
		child, err := tx.Children.FindByID(txCtx, guardianship.Child)
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

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		userIDObj, err := parseUserID(userID)
		if err != nil {
			return err
		}

		var guardianships []*domain.Guardianship
		if includeRevoked {
			guardianships, err = tx.Guardianships.FindByUserIDIncludingRevoked(txCtx, userIDObj)
		} else {
			guardianships, err = tx.Guardianships.FindByUserID(txCtx, userIDObj)
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
			child, err := tx.Children.FindByID(txCtx, g.Child)
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

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		childIDObj, err := parseChildID(childID)
		if err != nil {
			return err
		}

		var guardianships []*domain.Guardianship
		if includeRevoked {
			guardianships, err = tx.Guardianships.FindByChildIDIncludingRevoked(txCtx, childIDObj)
		} else {
			guardianships, err = tx.Guardianships.FindByChildID(txCtx, childIDObj)
		}
		if err != nil {
			return err
		}

		// 查询儿童信息（所有监护关系共享同一个儿童）
		child, err := tx.Children.FindByID(txCtx, childIDObj)
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
