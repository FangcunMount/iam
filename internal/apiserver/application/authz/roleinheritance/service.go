package roleinheritance

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	policychange "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policychange"
	authzuow "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/uow"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

type CreateCommand struct {
	TenantID        string
	RoleID          meta.ID
	InheritedRoleID meta.ID
	GrantedBy       string
}

type RevokeCommand struct {
	TenantID  string
	ID        meta.ID
	RevokedBy string
	Reason    string
}

type Service struct {
	uow      authzuow.UnitOfWork
	repo     domain.Repository
	reloader policychange.RuntimePolicyReloader
}

func NewService(uow authzuow.UnitOfWork, repo domain.Repository, reloader policychange.RuntimePolicyReloader) *Service {
	return &Service{uow: uow, repo: repo, reloader: reloader}
}

func (s *Service) Create(ctx context.Context, cmd CreateCommand) (*domain.Inheritance, error) {
	if s == nil || s.uow == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "role inheritance service is unavailable")
	}
	cmd.TenantID = strings.TrimSpace(cmd.TenantID)
	cmd.GrantedBy = strings.TrimSpace(cmd.GrantedBy)
	inheritance, err := domain.New(cmd.RoleID, cmd.InheritedRoleID, cmd.TenantID, cmd.GrantedBy)
	if err != nil {
		return nil, err
	}
	err = s.uow.WithinTx(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) error {
		if err := tx.RoleInheritances.CreateChecked(txCtx, &inheritance); err != nil {
			return err
		}
		version, err := tx.PolicyVersions.Increment(txCtx, cmd.TenantID, cmd.GrantedBy, "role inheritance created")
		if err != nil {
			return err
		}
		return policychange.StagePolicyVersionChanged(txCtx, tx.Events, cmd.TenantID, version)
	})
	if err != nil {
		return nil, err
	}
	policychange.ReloadRuntimePolicy(ctx, s.reloader, "role_inheritance_created")
	return &inheritance, nil
}

func (s *Service) Revoke(ctx context.Context, cmd RevokeCommand) error {
	if s == nil || s.uow == nil {
		return perrors.WithCode(code.ErrInternalServerError, "role inheritance service is unavailable")
	}
	cmd.TenantID = strings.TrimSpace(cmd.TenantID)
	cmd.RevokedBy = strings.TrimSpace(cmd.RevokedBy)
	if cmd.TenantID == "" || cmd.ID.IsZero() || cmd.RevokedBy == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "tenant, inheritance id, and revoked by are required")
	}
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) error {
		inheritance, err := tx.RoleInheritances.FindByID(txCtx, cmd.ID)
		if err != nil {
			return err
		}
		if inheritance.TenantIDString() != cmd.TenantID {
			return perrors.WithCode(code.ErrInvalidArgument, "role inheritance does not belong to tenant")
		}
		outcome, err := tx.RoleInheritances.AtomicRevoke(txCtx, cmd.ID, cmd.TenantID)
		if err != nil {
			return err
		}
		if outcome != domain.RevokeOutcomeRevoked {
			if outcome == domain.RevokeOutcomeAlreadyRevoked {
				return perrors.WithCode(code.ErrInvalidArgument, "role inheritance is not active")
			}
			return perrors.WithCode(code.ErrInvalidArgument, "role inheritance not found")
		}
		reason := strings.TrimSpace(cmd.Reason)
		if reason == "" {
			reason = "role inheritance revoked"
		}
		version, err := tx.PolicyVersions.Increment(txCtx, cmd.TenantID, cmd.RevokedBy, reason)
		if err != nil {
			return err
		}
		return policychange.StagePolicyVersionChanged(txCtx, tx.Events, cmd.TenantID, version)
	})
	if err != nil {
		return err
	}
	policychange.ReloadRuntimePolicy(ctx, s.reloader, "role_inheritance_revoked")
	return nil
}

func (s *Service) List(ctx context.Context, tenantID string, roleID meta.ID) ([]*domain.Inheritance, error) {
	if s == nil || s.repo == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "role inheritance repository is unavailable")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "tenant is required")
	}
	items, err := s.repo.ListActiveByTenant(ctx, tenantID)
	if err != nil || roleID.IsZero() {
		return items, err
	}
	filtered := make([]*domain.Inheritance, 0, len(items))
	for _, item := range items {
		if item != nil && item.RoleID == roleID {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}
