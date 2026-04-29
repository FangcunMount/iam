// Package policy 策略领域包
package policy

import (
	"context"
)

// RuleStore 持久化 Casbin 规则事实到数据库。
type RuleStore interface {
	AddPolicy(ctx context.Context, rules ...PolicyRule) error
	RemovePolicy(ctx context.Context, rules ...PolicyRule) error
	AddGroupingPolicy(ctx context.Context, rules ...GroupingRule) error
	RemoveGroupingPolicy(ctx context.Context, rules ...GroupingRule) error
}

// CasbinAdapter Casbin 策略操作接口（Driven Port - 外部服务）
type CasbinAdapter interface {
	RuleStore
	// GetPoliciesByRole 获取角色的所有 p 规则
	GetPoliciesByRole(ctx context.Context, role, domain string) ([]PolicyRule, error)
	// GetGroupingsBySubject 获取主体的所有 g 规则
	GetGroupingsBySubject(ctx context.Context, subject, domain string) ([]GroupingRule, error)
	// LoadPolicy 重新加载策略（用于缓存刷新）
	LoadPolicy(ctx context.Context) error

	// Enforce 执行 Casbin 判定（sub, dom, obj, act 与模型 request 定义一致）
	Enforce(ctx context.Context, sub, dom, obj, act string) (bool, error)
	// GetRolesForUser 返回用户在租户域下的直接角色键列表（如 role:admin）
	GetRolesForUser(ctx context.Context, user, domain string) ([]string, error)
	// GetImplicitRolesForUser 返回用户在租户域下的隐式角色键列表（包含继承角色）。
	GetImplicitRolesForUser(ctx context.Context, user, domain string) ([]string, error)
	// GetImplicitPermissionsForUser 返回用户在租户域下的隐式权限规则。
	GetImplicitPermissionsForUser(ctx context.Context, user, domain string) ([]PolicyRule, error)
}

// VersionNotifier 策略版本通知订阅接口（Driven Port - 外部服务）
type VersionNotifier interface {
	// Subscribe 订阅策略版本变更通知
	// handler 会在接收到版本变更通知时被调用
	Subscribe(ctx context.Context, handler VersionChangeHandler) error

	// Close 关闭订阅
	Close() error
}

// VersionChangeHandler 版本变更处理函数
type VersionChangeHandler func(tenantID string, version int64)
