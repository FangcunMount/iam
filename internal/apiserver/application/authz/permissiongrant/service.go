package permissiongrant

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authzshared "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/shared"
	authzuow "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/uow"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

type RuntimeReloader interface{ LoadPolicy(context.Context) error }

type CreateCommand struct {
	TenantID    string
	RoleID      meta.ID
	ResourceID  resource.ResourceID
	Action      string
	Constraints constraint.Set
	GrantedBy   string
}

type RevokeCommand struct {
	TenantID  string
	GrantID   meta.ID
	RevokedBy string
	Reason    string
}

type Service struct {
	uow      authzuow.UnitOfWork
	repo     domain.Repository
	reloader RuntimeReloader
}

func NewService(uow authzuow.UnitOfWork, repo domain.Repository, reloader RuntimeReloader) *Service {
	return &Service{uow: uow, repo: repo, reloader: reloader}
}

func (s *Service) Create(ctx context.Context, cmd CreateCommand) (*domain.Grant, error) {
	if s == nil || s.uow == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "permission grant service is unavailable")
	}
	cmd.TenantID = strings.TrimSpace(cmd.TenantID)
	cmd.GrantedBy = strings.TrimSpace(cmd.GrantedBy)
	if cmd.TenantID == "" || cmd.RoleID.IsZero() || cmd.ResourceID.Uint64() == 0 || cmd.GrantedBy == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "tenant, role, resource, and granted by are required")
	}
	var created domain.Grant
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) error {
		role, err := tx.Roles.FindByIDForUpdate(txCtx, cmd.RoleID)
		if err != nil {
			return err
		}
		if !role.BelongsToTenant(cmd.TenantID) {
			return perrors.WithCode(code.ErrInvalidArgument, "role does not belong to tenant")
		}
		catalogResource, err := tx.Resources.FindByIDForUpdate(txCtx, cmd.ResourceID)
		if err != nil {
			return err
		}
		grant, err := domain.New(
			cmd.RoleID, cmd.TenantID, cmd.ResourceID, catalogResource.KeyString(),
			cmd.Action, cmd.Constraints, cmd.GrantedBy,
		)
		if err != nil {
			return err
		}
		if err := grant.ValidateAgainst(*catalogResource); err != nil {
			return err
		}
		if err := tx.PermissionGrants.Create(txCtx, &grant); err != nil {
			return err
		}
		version, err := tx.PolicyVersions.Increment(txCtx, cmd.TenantID, cmd.GrantedBy, "permission grant created")
		if err != nil {
			return err
		}
		if err := authzshared.StagePolicyVersionChanged(txCtx, tx.Events, cmd.TenantID, version); err != nil {
			return err
		}
		created = grant
		return nil
	})
	if err != nil {
		return nil, err
	}
	authzshared.ReloadRuntimePolicy(ctx, s.reloader, "permission_grant_created")
	return &created, nil
}

func (s *Service) Revoke(ctx context.Context, cmd RevokeCommand) error {
	if s == nil || s.uow == nil {
		return perrors.WithCode(code.ErrInternalServerError, "permission grant service is unavailable")
	}
	cmd.TenantID = strings.TrimSpace(cmd.TenantID)
	cmd.RevokedBy = strings.TrimSpace(cmd.RevokedBy)
	if cmd.TenantID == "" || cmd.GrantID.IsZero() || cmd.RevokedBy == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "tenant, grant id, and revoked by are required")
	}
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) error {
		grant, err := tx.PermissionGrants.FindByID(txCtx, cmd.GrantID)
		if err != nil {
			return err
		}
		if grant.TenantIDString() != cmd.TenantID {
			return perrors.WithCode(code.ErrInvalidArgument, "permission grant does not belong to tenant")
		}
		if err := tx.PermissionGrants.Revoke(txCtx, cmd.GrantID); err != nil {
			return err
		}
		reason := strings.TrimSpace(cmd.Reason)
		if reason == "" {
			reason = "permission grant revoked"
		}
		version, err := tx.PolicyVersions.Increment(txCtx, cmd.TenantID, cmd.RevokedBy, reason)
		if err != nil {
			return err
		}
		return authzshared.StagePolicyVersionChanged(txCtx, tx.Events, cmd.TenantID, version)
	})
	if err != nil {
		return err
	}
	authzshared.ReloadRuntimePolicy(ctx, s.reloader, "permission_grant_revoked")
	return nil
}

func (s *Service) ListByRole(ctx context.Context, roleID meta.ID, tenantID string) ([]*domain.Grant, error) {
	if s == nil || s.repo == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "permission grant repository is unavailable")
	}
	if roleID.IsZero() || strings.TrimSpace(tenantID) == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "role id and tenant are required")
	}
	return s.repo.ListByRole(ctx, roleID, tenantID)
}
