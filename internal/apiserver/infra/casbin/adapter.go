package casbin

import (
	"context"
	"sync"
	"time"

	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

// CasbinAdapter Casbin 适配器实现
type CasbinAdapter struct {
	enforcer      *casbin.CachedEnforcer
	mu            sync.RWMutex
	lastReloadErr error
	lastReloadAt  time.Time
}

var _ domain.CasbinAdapter = (*CasbinAdapter)(nil)

// NewCasbinAdapter 创建 Casbin 适配器
func NewCasbinAdapter(db *gorm.DB, modelPath string) (domain.CasbinAdapter, error) {
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, err
	}

	enforcer, err := casbin.NewCachedEnforcer(modelPath, adapter)
	if err != nil {
		return nil, err
	}

	// DB 是授权事实源；运行时 Enforcer 只负责内存加载与判定。
	enforcer.EnableAutoSave(false)

	// 加载策略
	if err := enforcer.LoadPolicy(); err != nil {
		return nil, err
	}

	return &CasbinAdapter{
		enforcer: enforcer,
	}, nil
}

// AddPolicy 添加 p 规则
func (c *CasbinAdapter) AddPolicy(ctx context.Context, rules ...domain.PolicyRule) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, rule := range rules {
		_, err := c.enforcer.AddPolicy(rule.Sub, rule.Dom, rule.Obj, rule.Act)
		if err != nil {
			return err
		}
	}
	return nil
}

// RemovePolicy 删除 p 规则
func (c *CasbinAdapter) RemovePolicy(ctx context.Context, rules ...domain.PolicyRule) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, rule := range rules {
		_, err := c.enforcer.RemovePolicy(rule.Sub, rule.Dom, rule.Obj, rule.Act)
		if err != nil {
			return err
		}
	}
	return nil
}

// AddGroupingPolicy 添加 g 规则
func (c *CasbinAdapter) AddGroupingPolicy(ctx context.Context, rules ...domain.GroupingRule) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, rule := range rules {
		_, err := c.enforcer.AddGroupingPolicy(rule.Sub, rule.Role, rule.Dom)
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveGroupingPolicy 删除 g 规则
func (c *CasbinAdapter) RemoveGroupingPolicy(ctx context.Context, rules ...domain.GroupingRule) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, rule := range rules {
		_, err := c.enforcer.RemoveGroupingPolicy(rule.Sub, rule.Role, rule.Dom)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetPoliciesByRole 获取角色的所有 p 规则
func (c *CasbinAdapter) GetPoliciesByRole(ctx context.Context, role, domainStr string) ([]domain.PolicyRule, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	policies, err := c.enforcer.GetFilteredPolicy(0, role, domainStr)
	if err != nil {
		return nil, err
	}
	rules := make([]domain.PolicyRule, 0, len(policies))

	for _, p := range policies {
		if len(p) >= 4 {
			rules = append(rules, domain.PolicyRule{
				Sub: p[0],
				Dom: p[1],
				Obj: p[2],
				Act: p[3],
			})
		}
	}

	return rules, nil
}

// GetGroupingsBySubject 获取主体的所有 g 规则
func (c *CasbinAdapter) GetGroupingsBySubject(ctx context.Context, subject, domainStr string) ([]domain.GroupingRule, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	groupings, err := c.enforcer.GetFilteredGroupingPolicy(0, subject, "", domainStr)
	if err != nil {
		return nil, err
	}
	rules := make([]domain.GroupingRule, 0, len(groupings))

	for _, g := range groupings {
		if len(g) >= 3 {
			rules = append(rules, domain.GroupingRule{
				Sub:  g[0],
				Role: g[1],
				Dom:  g[2],
			})
		}
	}

	return rules, nil
}

// LoadPolicy 重新加载策略（用于缓存刷新）
func (c *CasbinAdapter) LoadPolicy(ctx context.Context) error {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()

	_ = c.enforcer.InvalidateCache()
	err := c.enforcer.LoadPolicy()
	c.lastReloadAt = time.Now()
	c.lastReloadErr = err
	return err
}

// Enforce 执行 Casbin 判定。
func (c *CasbinAdapter) Enforce(ctx context.Context, sub, dom, obj, act string) (bool, error) {
	_ = ctx
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.enforcer.Enforce(sub, dom, obj, act)
}

// GetRolesForUser 返回用户在指定租户域下的直接角色键。
func (c *CasbinAdapter) GetRolesForUser(ctx context.Context, user, domain string) ([]string, error) {
	_ = ctx
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.enforcer.GetRolesForUser(user, domain)
}

// GetImplicitRolesForUser 返回用户在指定租户域下的隐式角色键。
func (c *CasbinAdapter) GetImplicitRolesForUser(ctx context.Context, user, domain string) ([]string, error) {
	_ = ctx
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.enforcer.GetImplicitRolesForUser(user, domain)
}

// GetImplicitPermissionsForUser 返回用户在指定租户域下的隐式权限规则。
func (c *CasbinAdapter) GetImplicitPermissionsForUser(ctx context.Context, user, dom string) ([]domain.PolicyRule, error) {
	_ = ctx
	c.mu.RLock()
	defer c.mu.RUnlock()

	permissions, err := c.enforcer.GetImplicitPermissionsForUser(user, dom)
	if err != nil {
		return nil, err
	}

	rules := make([]domain.PolicyRule, 0, len(permissions))
	for _, permission := range permissions {
		if len(permission) < 4 {
			continue
		}
		rules = append(rules, domain.PolicyRule{
			Sub: permission[0],
			Dom: permission[1],
			Obj: permission[2],
			Act: permission[3],
		})
	}
	return rules, nil
}

// Enforcer 获取 Enforcer 实例（用于 PEP）
func (c *CasbinAdapter) Enforcer() *casbin.CachedEnforcer {
	return c.enforcer
}

// InvalidateCache 清除缓存
func (c *CasbinAdapter) InvalidateCache() {
	_ = c.enforcer.InvalidateCache()
}

// ReloadHealth 返回最近一次策略加载结果。
func (c *CasbinAdapter) ReloadHealth() (bool, error, time.Time) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastReloadErr == nil, c.lastReloadErr, c.lastReloadAt
}
