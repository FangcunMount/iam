// Package assignment 赋权命令应用服务
package assignment

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	authzshared "github.com/FangcunMount/iam/internal/apiserver/application/authz/shared"
	authzuow "github.com/FangcunMount/iam/internal/apiserver/application/authz/uow"
	assignmentDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/assignment"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// AssignmentCommandService 赋权命令服务（实现 AssignmentCommander 接口）
// 负责协调领域服务、仓储和外部端口，处理赋权的写操作
// 核心职责：管理数据库和 Casbin 的双写事务一致性
type AssignmentCommandService struct {
	assignmentValidator assignmentDomain.Validator
	uow                 authzuow.UnitOfWork
	casbinAdapter       policyDomain.CasbinAdapter
	versionNotifier     policyDomain.VersionNotifier
}

// NewAssignmentCommandService 创建赋权命令服务
func NewAssignmentCommandService(
	assignmentValidator assignmentDomain.Validator,
	uow authzuow.UnitOfWork,
	casbinAdapter policyDomain.CasbinAdapter,
	versionNotifier policyDomain.VersionNotifier,
) *AssignmentCommandService {
	return &AssignmentCommandService{
		assignmentValidator: assignmentValidator,
		uow:                 uow,
		casbinAdapter:       casbinAdapter,
		versionNotifier:     versionNotifier,
	}
}

// Grant 授权（赋予角色）
func (s *AssignmentCommandService) Grant(ctx context.Context, cmd assignmentDomain.GrantCommand) (*assignmentDomain.Assignment, error) {
	// 1. 验证授权命令
	if err := s.assignmentValidator.ValidateGrantCommand(cmd); err != nil {
		return nil, err
	}

	var (
		newAssignment *assignmentDomain.Assignment
		version       *policyDomain.PolicyVersion
	)

	err := s.uow.WithinTx(ctx, func(tx authzuow.TxRepositories) error {
		txValidator := assignmentDomain.NewValidator(tx.Assignments, tx.Roles, tx.Users)
		if err := txValidator.CheckRoleExists(ctx, cmd.RoleID, cmd.TenantID); err != nil {
			return err
		}
		if err := txValidator.CheckSubjectExists(ctx, cmd.SubjectType, cmd.SubjectID, cmd.TenantID); err != nil {
			return err
		}

		role, err := tx.Roles.FindByID(ctx, meta.FromUint64(cmd.RoleID))
		if err != nil {
			return errors.Wrap(err, "获取角色失败")
		}
		if role.TenantID != cmd.TenantID {
			return errors.New("角色不属于当前租户")
		}

		created := assignmentDomain.NewAssignment(
			cmd.SubjectType,
			cmd.SubjectID,
			cmd.RoleID,
			cmd.TenantID,
			assignmentDomain.WithGrantedBy(cmd.GrantedBy),
		)
		if err := tx.Assignments.Create(ctx, &created); err != nil {
			return errors.Wrap(err, "创建赋权失败")
		}

		groupingRule := policyDomain.GroupingRule{
			Sub:  created.SubjectKey(),
			Role: role.Key(),
			Dom:  cmd.TenantID,
		}
		if err := tx.RuleStore.AddGroupingPolicy(ctx, groupingRule); err != nil {
			return errors.Wrap(err, "添加 Casbin 分组规则失败")
		}

		version, err = tx.PolicyVersions.Increment(ctx, cmd.TenantID, cmd.GrantedBy, "assignment grant")
		if err != nil {
			return errors.Wrap(err, "更新授权版本失败")
		}

		newAssignment = &created
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.publishVersion(ctx, cmd.TenantID, version)
	authzshared.ReloadRuntimePolicy(ctx, s.casbinAdapter, "assignment grant")
	return newAssignment, nil
}

// Revoke 撤销授权（移除角色）
func (s *AssignmentCommandService) Revoke(ctx context.Context, cmd assignmentDomain.RevokeCommand) error {
	// 1. 验证撤销命令
	if err := s.assignmentValidator.ValidateRevokeCommand(cmd); err != nil {
		return err
	}

	var version *policyDomain.PolicyVersion
	err := s.uow.WithinTx(ctx, func(tx authzuow.TxRepositories) error {
		role, err := tx.Roles.FindByID(ctx, meta.FromUint64(cmd.RoleID))
		if err != nil {
			return errors.Wrap(err, "获取角色失败")
		}
		if role.TenantID != cmd.TenantID {
			return errors.New("角色不属于当前租户")
		}

		if err := tx.Assignments.DeleteBySubjectAndRole(ctx, cmd.SubjectType, cmd.SubjectID, cmd.RoleID, cmd.TenantID); err != nil {
			return errors.Wrap(err, "删除赋权记录失败")
		}

		groupingRule := policyDomain.GroupingRule{
			Sub:  string(cmd.SubjectType) + ":" + cmd.SubjectID,
			Role: role.Key(),
			Dom:  cmd.TenantID,
		}
		if err := tx.RuleStore.RemoveGroupingPolicy(ctx, groupingRule); err != nil {
			return errors.Wrap(err, "删除 Casbin 分组规则失败")
		}

		version, err = tx.PolicyVersions.Increment(ctx, cmd.TenantID, "system", "assignment revoke")
		if err != nil {
			return errors.Wrap(err, "更新授权版本失败")
		}
		return nil
	})
	if err != nil {
		return err
	}

	s.publishVersion(ctx, cmd.TenantID, version)
	authzshared.ReloadRuntimePolicy(ctx, s.casbinAdapter, "assignment revoke")
	return nil
}

// RevokeByID 根据ID撤销授权
func (s *AssignmentCommandService) RevokeByID(ctx context.Context, cmd assignmentDomain.RevokeByIDCommand) error {
	var (
		version          *policyDomain.PolicyVersion
		targetAssignment *assignmentDomain.Assignment
	)

	err := s.uow.WithinTx(ctx, func(tx authzuow.TxRepositories) error {
		var err error
		targetAssignment, err = tx.Assignments.FindByID(ctx, cmd.AssignmentID)
		if err != nil {
			return errors.Wrap(err, "获取赋权记录失败")
		}
		if targetAssignment.TenantID != cmd.TenantID {
			return errors.New("赋权记录不属于当前租户")
		}

		role, err := tx.Roles.FindByID(ctx, meta.FromUint64(targetAssignment.RoleID))
		if err != nil {
			return errors.Wrap(err, "获取角色失败")
		}

		groupingRule := policyDomain.GroupingRule{
			Sub:  targetAssignment.SubjectKey(),
			Role: role.Key(),
			Dom:  targetAssignment.TenantID,
		}
		if err := tx.RuleStore.RemoveGroupingPolicy(ctx, groupingRule); err != nil {
			return errors.Wrap(err, "删除 Casbin 分组规则失败")
		}
		if err := tx.Assignments.Delete(ctx, targetAssignment.ID); err != nil {
			return errors.Wrap(err, "删除赋权记录失败")
		}

		version, err = tx.PolicyVersions.Increment(ctx, targetAssignment.TenantID, "system", "assignment revoke")
		if err != nil {
			return errors.Wrap(err, "更新授权版本失败")
		}
		return nil
	})
	if err != nil {
		return err
	}

	s.publishVersion(ctx, cmd.TenantID, version)
	authzshared.ReloadRuntimePolicy(ctx, s.casbinAdapter, "assignment revoke by id")
	return nil
}

func (s *AssignmentCommandService) publishVersion(ctx context.Context, tenantID string, version *policyDomain.PolicyVersion) {
	if s.versionNotifier == nil || version == nil {
		return
	}
	if err := s.versionNotifier.Publish(ctx, tenantID, version.Version); err != nil {
		log.Errorw("failed to publish authz assignment version", "tenant_id", tenantID, "version", version.Version, "error", err)
	}
}
