package policy

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authzshared "github.com/FangcunMount/iam/internal/apiserver/application/authz/shared"
	authzuow "github.com/FangcunMount/iam/internal/apiserver/application/authz/uow"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

type PolicyChangeBuilder func(ctx context.Context, tx authzuow.TxRepositories) (policyDomain.PolicyChange, error)

type PolicyChangeMutation func(ctx context.Context, tx authzuow.TxRepositories, change policyDomain.PolicyChange) error

type RuntimePolicyReloader = authzshared.RuntimePolicyReloader

type commitOptions struct {
	beforeFacts []PolicyChangeMutation
	afterFacts  []PolicyChangeMutation
}

type CommitOption func(*commitOptions)

func BeforeFacts(mutation PolicyChangeMutation) CommitOption {
	return func(opts *commitOptions) {
		opts.beforeFacts = append(opts.beforeFacts, mutation)
	}
}

func AfterFacts(mutation PolicyChangeMutation) CommitOption {
	return func(opts *commitOptions) {
		opts.afterFacts = append(opts.afterFacts, mutation)
	}
}

// PolicyChangeCommitter owns the application transaction for authorization policy changes.
type PolicyChangeCommitter struct {
	uow             authzuow.UnitOfWork
	runtimeReloader RuntimePolicyReloader
}

func NewPolicyChangeCommitter(uow authzuow.UnitOfWork, runtimeReloader RuntimePolicyReloader) *PolicyChangeCommitter {
	return &PolicyChangeCommitter{uow: uow, runtimeReloader: runtimeReloader}
}

func (c *PolicyChangeCommitter) Commit(ctx context.Context, build PolicyChangeBuilder, opts ...CommitOption) error {
	if c == nil || c.uow == nil {
		return perrors.WithCode(code.ErrInternalServerError, "authorization policy committer unavailable")
	}
	if build == nil {
		return perrors.WithCode(code.ErrInternalServerError, "authorization policy change builder unavailable")
	}

	options := commitOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	var committed policyDomain.PolicyChange
	err := c.uow.WithinTx(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) error {
		change, err := build(txCtx, tx)
		if err != nil {
			return err
		}
		for _, mutation := range options.beforeFacts {
			if err := mutation(txCtx, tx, change); err != nil {
				return err
			}
		}
		if err := writeAuthorizationFact(txCtx, tx, change); err != nil {
			return err
		}
		for _, mutation := range options.afterFacts {
			if err := mutation(txCtx, tx, change); err != nil {
				return err
			}
		}
		version, err := tx.PolicyVersions.Increment(txCtx, change.TenantID, change.Actor.ID, change.Reason)
		if err != nil {
			return err
		}
		if err := authzshared.StagePolicyVersionChanged(txCtx, tx.Events, change.TenantID, version); err != nil {
			return err
		}
		committed = change
		return nil
	})
	if err != nil {
		return err
	}

	authzshared.ReloadRuntimePolicy(ctx, c.runtimeReloader, string(committed.Kind))
	return nil
}

func writeAuthorizationFact(txCtx context.Context, tx authzuow.TxRepositories, change policyDomain.PolicyChange) error {
	switch change.Kind {
	case policyDomain.PolicyChangeGrantPermission:
		if change.Permission == nil {
			return perrors.WithCode(code.ErrInternalServerError, "permission change missing permission")
		}
		return tx.AuthorizationFacts.AddPermission(txCtx, *change.Permission)
	case policyDomain.PolicyChangeRevokePermission:
		if change.Permission == nil {
			return perrors.WithCode(code.ErrInternalServerError, "permission change missing permission")
		}
		return tx.AuthorizationFacts.RemovePermission(txCtx, *change.Permission)
	case policyDomain.PolicyChangeBindRole:
		if change.RoleBinding == nil {
			return perrors.WithCode(code.ErrInternalServerError, "role binding change missing binding")
		}
		return tx.AuthorizationFacts.AddRoleBinding(txCtx, *change.RoleBinding)
	case policyDomain.PolicyChangeUnbindRole:
		if change.RoleBinding == nil {
			return perrors.WithCode(code.ErrInternalServerError, "role binding change missing binding")
		}
		return tx.AuthorizationFacts.RemoveRoleBinding(txCtx, *change.RoleBinding)
	default:
		return perrors.WithCode(code.ErrInvalidArgument, "unsupported authorization policy change kind: %s", change.Kind)
	}
}
