package permissiongrant

import (
	"context"
	"strings"

	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/management"

	perrors "github.com/FangcunMount/component-base/pkg/errors"

	policychange "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policychange"
	authzuow "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/uow"
	domain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v5/internal/pkg/code"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
)

type CreateCommand struct {
	RoleID     meta.ID
	ResourceID resource.ResourceID
	Action     string
	GrantedBy  string
}

type RevokeCommand struct {
	GrantID   meta.ID
	RevokedBy string
	Reason    string
}

type Service struct {
	guard    management.Guard
	uow      authzuow.UnitOfWork
	reader   authzuow.ReadOnlyPlanner
	repo     domain.Repository
	reloader policychange.RuntimePolicyReloader
}

func NewService(uow authzuow.UnitOfWork, repo domain.Repository, reloader policychange.RuntimePolicyReloader, guard management.Guard) *Service {
	reader, _ := uow.(authzuow.ReadOnlyPlanner)
	return &Service{uow: uow, reader: reader, repo: repo, reloader: reloader, guard: guard}
}

func (s *Service) Create(ctx context.Context, cmd CreateCommand) (*domain.Grant, error) {
	if err := s.guard.RequireOperation(ctx, "iam:authz:collection:permission_grants", "create"); err != nil {
		return nil, err
	}
	if s == nil || s.uow == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "permission grant service is unavailable")
	}
	cmd.GrantedBy = strings.TrimSpace(cmd.GrantedBy)
	cmd.Action = strings.TrimSpace(cmd.Action)
	if cmd.RoleID.IsZero() || cmd.ResourceID.Uint64() == 0 || cmd.GrantedBy == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "role, resource, and granted by are required")
	}
	var created domain.Grant
	err := authzuow.WithinWrite(ctx, s.uow, s.previewCreate(cmd, &created), func(txCtx context.Context, tx authzuow.TxRepositories) error {
		if _, planned := policychange.ExecutionPlan(txCtx); planned {
			if err := s.guard.RequireOperation(txCtx, grantOperations, "create"); err != nil {
				return err
			}
		}
		role, err := tx.Roles.FindByIDForUpdate(txCtx, cmd.RoleID)
		if err != nil {
			return err
		}
		catalogResource, err := tx.Resources.FindByIDForUpdate(txCtx, cmd.ResourceID)
		if err != nil {
			return err
		}
		if err := s.guard.Require(txCtx, role); err != nil {
			return err
		}
		grant, err := domain.New(
			cmd.RoleID, cmd.ResourceID, catalogResource.KeyString(),
			cmd.Action, cmd.GrantedBy,
		)
		if err != nil {
			return err
		}
		if err := role.ValidateGrant(grant.ResourceKey, grant.Action); err != nil {
			return err
		}
		if err := grant.ValidateAgainst(*catalogResource); err != nil {
			return err
		}

		if err := tx.PermissionGrants.Create(txCtx, &grant); err != nil {
			return err
		}
		version, err := tx.PolicyVersions.Increment(txCtx, cmd.GrantedBy, "permission grant created")
		if err != nil {
			return err
		}
		if err := policychange.StagePolicyVersionChanged(txCtx, tx.Events, version); err != nil {
			return err
		}
		created = grant
		return nil
	})
	if err != nil {
		return nil, err
	}
	policychange.ReloadRuntimePolicy(ctx, s.reloader, "permission_grant_created")
	return &created, nil
}

func (s *Service) Revoke(ctx context.Context, cmd RevokeCommand) error {
	if err := s.guard.RequireOperation(ctx, "iam:authz:collection:permission_grants", "revoke"); err != nil {
		return err
	}
	if s == nil || s.uow == nil {
		return perrors.WithCode(code.ErrInternalServerError, "permission grant service is unavailable")
	}
	cmd.RevokedBy = strings.TrimSpace(cmd.RevokedBy)
	if cmd.GrantID.IsZero() || cmd.RevokedBy == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "grant id and revoked by are required")
	}
	cmd.Reason = strings.TrimSpace(cmd.Reason)
	if cmd.Reason == "" {
		cmd.Reason = "permission grant revoked"
	}
	revoked := false
	err := authzuow.WithinWrite(ctx, s.uow, s.previewRevoke(cmd), func(txCtx context.Context, tx authzuow.TxRepositories) error {
		if _, planned := policychange.ExecutionPlan(txCtx); planned {
			if err := s.guard.RequireOperation(txCtx, grantOperations, "revoke"); err != nil {
				return err
			}
		}
		roleID, err := tx.PermissionGrants.FindRoleID(txCtx, cmd.GrantID)
		if err != nil {
			return err
		}
		target, err := tx.Roles.FindByIDForUpdate(txCtx, roleID)
		if err != nil {
			return err
		}
		if err := s.guard.Require(txCtx, target); err != nil {
			return err
		}
		outcome, err := tx.PermissionGrants.AtomicRevoke(txCtx, cmd.GrantID)
		if err != nil {
			return err
		}
		switch outcome {
		case domain.RevokeOutcomeRevoked, domain.RevokeOutcomeAlreadyRevoked:
			if !outcome.AppliesVersionChange() {
				return nil
			}
			revoked = true
		case domain.RevokeOutcomeNotFound:
			return perrors.WithCode(code.ErrInvalidArgument, "permission grant not found")
		default:
			return perrors.WithCode(code.ErrInternalServerError, "unexpected permission grant revoke outcome")
		}
		reason := strings.TrimSpace(cmd.Reason)
		if reason == "" {
			reason = "permission grant revoked"
		}
		version, err := tx.PolicyVersions.Increment(txCtx, cmd.RevokedBy, reason)
		if err != nil {
			return err
		}
		return policychange.StagePolicyVersionChanged(txCtx, tx.Events, version)
	})
	if err != nil {
		return err
	}
	if revoked {
		policychange.ReloadRuntimePolicy(ctx, s.reloader, "permission_grant_revoked")
	}
	return nil
}

func (s *Service) ListByRole(ctx context.Context, roleID meta.ID) ([]*domain.Grant, error) {
	if s == nil || s.repo == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "permission grant repository is unavailable")
	}
	if roleID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "role id is required")
	}
	if s.reader == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "角色查询不可用")
	}
	var result []*domain.Grant
	err := s.reader.ReadPlan(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) error {
		target, err := tx.Roles.FindByID(txCtx, roleID)
		if err != nil {
			return err
		}
		if err = s.guard.RequireVisible(txCtx, target); err != nil {
			return err
		}
		result, err = tx.PermissionGrants.ListByRole(txCtx, roleID)
		return err
	})
	return result, err
}
