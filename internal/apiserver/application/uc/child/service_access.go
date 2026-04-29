package child

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// ============================================
// ==== ChildAccessApplicationService 实现 =====
// ============================================

type childAccessApplicationService struct {
	uow uow.UnitOfWork
}

// NewChildAccessApplicationService 创建当前监护人视角的儿童档案用例服务。
func NewChildAccessApplicationService(uow uow.UnitOfWork) ChildAccessApplicationService {
	return &childAccessApplicationService{uow: uow}
}

func (s *childAccessApplicationService) ListForGuardian(ctx context.Context, userID string) ([]*ChildResult, error) {
	var results []*ChildResult
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		userIDObj, err := parseChildAccessUserID(userID)
		if err != nil {
			return err
		}
		guardianships, err := tx.Guardianships.FindByUserID(txCtx, userIDObj)
		if err != nil {
			return err
		}
		results = make([]*ChildResult, 0, len(guardianships))
		for _, guardianship := range guardianships {
			if guardianship == nil {
				continue
			}
			child, err := tx.Children.FindByID(txCtx, guardianship.Child)
			if err != nil {
				return err
			}
			results = append(results, toChildResult(child))
		}
		return nil
	})
	return results, err
}

func (s *childAccessApplicationService) GetForGuardian(ctx context.Context, userID string, childID string) (*ChildResult, error) {
	if err := s.ensureActiveGuardianAccess(ctx, userID, childID); err != nil {
		return nil, err
	}
	return NewChildQueryApplicationService(s.uow).GetByID(ctx, childID)
}

func (s *childAccessApplicationService) PatchForGuardian(ctx context.Context, dto PatchChildForGuardianDTO) (*ChildResult, error) {
	if err := s.ensureActiveGuardianAccess(ctx, dto.UserID, dto.ChildID); err != nil {
		return nil, err
	}

	profile := NewChildProfileApplicationService(s.uow)
	if dto.LegalName != nil && strings.TrimSpace(*dto.LegalName) != "" {
		if err := profile.Rename(ctx, dto.ChildID, strings.TrimSpace(*dto.LegalName)); err != nil {
			return nil, err
		}
	}

	if dto.Gender != nil || dto.Birthday != nil {
		profileDTO := UpdateChildProfileDTO{ChildID: dto.ChildID}
		if dto.Gender != nil {
			profileDTO.Gender = *dto.Gender
		}
		if dto.Birthday != nil {
			profileDTO.Birthday = strings.TrimSpace(*dto.Birthday)
		}
		if err := profile.UpdateProfile(ctx, profileDTO); err != nil {
			return nil, err
		}
	}

	if dto.Height != nil || dto.Weight != nil {
		measurementDTO := UpdateHeightWeightDTO{ChildID: dto.ChildID}
		if dto.Height != nil {
			measurementDTO.Height = *dto.Height
		}
		if dto.Weight != nil {
			measurementDTO.Weight = *dto.Weight
		}
		if err := profile.UpdateHeightWeight(ctx, measurementDTO); err != nil {
			return nil, err
		}
	}

	return NewChildQueryApplicationService(s.uow).GetByID(ctx, dto.ChildID)
}

func (s *childAccessApplicationService) ensureActiveGuardianAccess(ctx context.Context, userID string, childID string) error {
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		userIDObj, err := parseChildAccessUserID(userID)
		if err != nil {
			return err
		}
		childIDObj, err := parseChildID(childID)
		if err != nil {
			return err
		}
		guardianship, err := tx.Guardianships.FindByUserIDAndChildID(txCtx, userIDObj, childIDObj)
		if err != nil {
			return err
		}
		if guardianship == nil {
			return perrors.WithCode(code.ErrPermissionDenied, "you are not the guardian of this child")
		}
		return nil
	})
	if err != nil {
		return perrors.WithCode(code.ErrPermissionDenied, "you are not the guardian of this child")
	}
	return nil
}
