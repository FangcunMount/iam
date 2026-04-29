package guardianship

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/input"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/guardianship"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ====================================================
// ==== GuardianshipAccessApplicationService 实现 =====
// ====================================================

type guardianshipAccessApplicationService struct {
	uow uow.UnitOfWork
}

// NewGuardianshipAccessApplicationService 创建当前用户视角的监护关系访问用例。
func NewGuardianshipAccessApplicationService(uow uow.UnitOfWork) GuardianshipAccessApplicationService {
	return &guardianshipAccessApplicationService{uow: uow}
}

func (s *guardianshipAccessApplicationService) GrantForCurrentUser(ctx context.Context, currentUserID string, dto AddGuardianDTO) (*GuardianshipResult, error) {
	if dto.UserID != "" && dto.UserID != currentUserID {
		return nil, perrors.WithCode(code.ErrPermissionDenied, "cannot grant guardianship for another user")
	}
	dto.UserID = currentUserID
	if err := NewGuardianshipApplicationService(s.uow).AddGuardian(ctx, dto); err != nil {
		return nil, err
	}
	return NewGuardianshipQueryApplicationService(s.uow).GetByUserIDAndChildID(ctx, currentUserID, dto.ChildID)
}

func (s *guardianshipAccessApplicationService) ListForCurrentUser(ctx context.Context, currentUserID string, dto ListGuardianshipsDTO) ([]*GuardianshipResult, error) {
	if dto.UserID != "" && dto.UserID != currentUserID {
		return nil, perrors.WithCode(code.ErrPermissionDenied, "cannot query guardianships for another user")
	}
	dto.UserID = currentUserID
	query := NewGuardianshipQueryApplicationService(s.uow)

	switch {
	case dto.UserID != "" && dto.ChildID != "":
		if err := ensureActiveGuardianAccess(ctx, query, currentUserID, dto.ChildID); err != nil {
			return nil, err
		}
		result, err := getByUserIDAndChildID(ctx, query, dto)
		if err != nil {
			return nil, err
		}
		if result == nil {
			return []*GuardianshipResult{}, nil
		}
		return []*GuardianshipResult{result}, nil
	case dto.UserID != "":
		return listChildrenByUserID(ctx, query, dto)
	case dto.ChildID != "":
		if err := ensureActiveGuardianAccess(ctx, query, currentUserID, dto.ChildID); err != nil {
			return nil, err
		}
		return listGuardiansByChildID(ctx, query, dto)
	default:
		return listChildrenByUserID(ctx, query, dto)
	}
}

func (s *guardianshipAccessApplicationService) RevokeBySelector(ctx context.Context, dto RevokeGuardianBySelectorDTO) (*GuardianshipResult, error) {
	var result *GuardianshipResult
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		var userID meta.ID
		var childID meta.ID
		if dto.GuardianshipID != "" {
			guardianshipID, err := input.ParseUserID(dto.GuardianshipID)
			if err != nil {
				return err
			}
			existing, err := tx.Guardianships.FindByID(ctx, guardianshipID)
			if err != nil {
				return err
			}
			userID = existing.User
			childID = existing.Child
		} else {
			parsedUserID, err := parseUserID(dto.UserID)
			if err != nil {
				return err
			}
			parsedChildID, err := parseChildID(dto.ChildID)
			if err != nil {
				return err
			}
			userID = parsedUserID
			childID = parsedChildID
		}

		managerService := domain.NewManagerService(tx.Guardianships, tx.Children, tx.Users)
		guardianship, err := managerService.RemoveGuardian(ctx, userID, childID)
		if err != nil {
			return err
		}
		if err := tx.Guardianships.Update(ctx, guardianship); err != nil {
			return err
		}

		child, err := tx.Children.FindByID(ctx, guardianship.Child)
		if err != nil {
			return err
		}
		result = toGuardianshipResult(guardianship, child)
		return nil
	})
	return result, err
}

func getByUserIDAndChildID(ctx context.Context, query GuardianshipQueryApplicationService, dto ListGuardianshipsDTO) (*GuardianshipResult, error) {
	if dto.Active != nil && !*dto.Active {
		return query.GetByUserIDAndChildIDIncludingRevoked(ctx, dto.UserID, dto.ChildID)
	}
	return query.GetByUserIDAndChildID(ctx, dto.UserID, dto.ChildID)
}

func listChildrenByUserID(ctx context.Context, query GuardianshipQueryApplicationService, dto ListGuardianshipsDTO) ([]*GuardianshipResult, error) {
	if dto.Active != nil && !*dto.Active {
		return query.ListChildrenByUserIDIncludingRevoked(ctx, dto.UserID)
	}
	return query.ListChildrenByUserID(ctx, dto.UserID)
}

func listGuardiansByChildID(ctx context.Context, query GuardianshipQueryApplicationService, dto ListGuardianshipsDTO) ([]*GuardianshipResult, error) {
	if dto.Active != nil && !*dto.Active {
		return query.ListGuardiansByChildIDIncludingRevoked(ctx, dto.ChildID)
	}
	return query.ListGuardiansByChildID(ctx, dto.ChildID)
}

func ensureActiveGuardianAccess(ctx context.Context, query GuardianshipQueryApplicationService, userID string, childID string) error {
	if _, err := query.GetByUserIDAndChildID(ctx, userID, childID); err != nil {
		return perrors.WithCode(code.ErrPermissionDenied, "you are not an active guardian of this child")
	}
	return nil
}
