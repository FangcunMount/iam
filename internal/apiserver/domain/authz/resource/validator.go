// Package service 资源领域服务
//
// 本包提供资源管理的领域服务，封装业务规则。
// 领域服务是内部实现细节，不对外暴露，仅被应用服务编排使用。
package resource

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// ResourceManager 资源管理领域服务
//
// 职责：
// - 封装资源相关的业务规则
// - 提供资源唯一性检查、参数验证等业务逻辑
// - 被应用服务编排使用，不对接口层直接暴露
//
// 设计原则：
// - 不实现 driving 接口（那是应用服务的职责）
// - 提供细粒度的业务规则方法
// - 无状态，所有依赖通过构造函数注入
//
// 注意：虽然是大写导出的，但在架构上只应该被应用层使用，不应该被接口层直接调用
type validator struct {
	resourceRepo Repository
}

// NewValidator 创建资源管理领域服务
func NewValidator(resourceRepo Repository) *validator {
	return &validator{
		resourceRepo: resourceRepo,
	}
}

// CheckKeyUnique 检查资源键的唯一性
//
// 业务规则：资源键在全局范围内必须唯一
func (v *validator) CheckKeyUnique(ctx context.Context, key string) error {
	if key == "" {
		return errors.WithCode(code.ErrInvalidArgument, "资源键不能为空")
	}

	// 查询是否已存在
	existingResource, err := v.resourceRepo.FindByKey(ctx, key)
	if err != nil && !errors.IsCode(err, code.ErrResourceNotFound) {
		return errors.Wrap(err, "检查资源键唯一性失败")
	}

	if existingResource != nil {
		return errors.WithCode(code.ErrResourceAlreadyExists, "资源键 %s 已存在", key)
	}

	return nil
}

// ValidateCreateParameters 验证创建资源的参数
//
// 业务规则：
// - Key 不能为空
// - DisplayName 不能为空
// - Actions 至少有一个
// - AppName、Domain、Type 都不能为空
func (v *validator) ValidateCreateParameters(key string, displayName string, appName string, domain string, resourceType string, actions []string) error {
	if _, err := NewKey(key); err != nil {
		return err
	}
	if displayName == "" {
		return errors.WithCode(code.ErrInvalidArgument, "显示名称不能为空")
	}
	if _, err := NormalizeActions(actions); err != nil {
		return err
	}
	if appName == "" {
		return errors.WithCode(code.ErrInvalidArgument, "应用名称不能为空")
	}
	if domain == "" {
		return errors.WithCode(code.ErrInvalidArgument, "业务域不能为空")
	}
	if resourceType == "" {
		return errors.WithCode(code.ErrInvalidArgument, "对象类型不能为空")
	}
	return nil
}

// ValidateUpdateParameters 验证更新资源的参数
//
// 业务规则：
// - 如果更新 Actions，至少要保留一个
func (v *validator) ValidateUpdateParameters(actions []string) error {
	if actions != nil {
		if _, err := NormalizeActions(actions); err != nil {
			return err
		}
	}
	return nil
}

func (v *validator) ValidateScopeKinds(kinds []scope.Kind) error {
	_, err := NormalizeAndValidateScopeKinds(kinds)
	return err
}

// CheckResourceExists 检查资源是否存在
func (v *validator) CheckResourceExists(ctx context.Context, resourceID ResourceID) (*Resource, error) {
	if resourceID.Uint64() == 0 {
		return nil, errors.WithCode(code.ErrInvalidArgument, "资源ID不能为空")
	}

	foundResource, err := v.resourceRepo.FindByID(ctx, resourceID)
	if err != nil {
		if errors.IsCode(err, code.ErrResourceNotFound) {
			return nil, errors.WithCode(code.ErrResourceNotFound, "资源 %d 不存在", resourceID.Uint64())
		}
		return nil, errors.Wrap(err, "获取资源失败")
	}

	return foundResource, nil
}
