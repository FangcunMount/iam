package casbin

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/decision"
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

// NewCasbinAdapter 创建 Casbin 适配器。
func NewCasbinAdapter(db *gorm.DB, modelPath string) (*CasbinAdapter, error) {
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, err
	}

	if err := normalizePersistedPolicyScopes(db); err != nil {
		return nil, err
	}

	enforcer, err := casbin.NewCachedEnforcer(modelPath, adapter)
	if err != nil {
		return nil, err
	}
	enforcer.AddFunction("scopeMatch", scopeMatchFunc)
	enforcer.AddFunction("resourceMatch", resourceMatchFunc)
	enforcer.AddFunction("actionMatch", actionMatchFunc)

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

func normalizePersistedPolicyScopes(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	return db.Exec(
		"UPDATE casbin_rule SET v4 = ? WHERE ptype = ? AND (v4 IS NULL OR v4 = '')",
		defaultScopeKey,
		"p",
	).Error
}

func (c *CasbinAdapter) addPolicyFacts(ctx context.Context, rules ...PolicyRule) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, rule := range rules {
		_, err := c.enforcer.AddPolicy(rule.Sub, rule.Dom, rule.Obj, rule.Act, normalizeScopeKey(rule.Scope))
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *CasbinAdapter) removePolicyFacts(ctx context.Context, rules ...PolicyRule) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, rule := range rules {
		_, err := c.enforcer.RemovePolicy(rule.Sub, rule.Dom, rule.Obj, rule.Act, normalizeScopeKey(rule.Scope))
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *CasbinAdapter) addGroupingFacts(ctx context.Context, rules ...GroupingRule) error {
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

// func (c *CasbinAdapter) removeGroupingFacts(ctx context.Context, rules ...GroupingRule) error {
// 	c.mu.Lock()
// 	defer c.mu.Unlock()

// 	for _, rule := range rules {
// 		_, err := c.enforcer.RemoveGroupingPolicy(rule.Sub, rule.Role, rule.Dom)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

func (c *CasbinAdapter) policyFactsForRole(ctx context.Context, roleName, domainStr string) ([]PolicyRule, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	policies, err := c.enforcer.GetFilteredPolicy(0, RoleKey(roleName), domainStr)
	if err != nil {
		return nil, err
	}
	rules := make([]PolicyRule, 0, len(policies))

	for _, p := range policies {
		if len(p) >= 4 {
			rules = append(rules, PolicyRule{
				Sub:   p[0],
				Dom:   p[1],
				Obj:   p[2],
				Act:   p[3],
				Scope: policyScopeAt(p, 4),
			})
		}
	}

	return rules, nil
}

// func (c *CasbinAdapter) groupingFactsForSubject(ctx context.Context, subject, domainStr string) ([]GroupingRule, error) {
// 	c.mu.RLock()
// 	defer c.mu.RUnlock()

// 	groupings, err := c.enforcer.GetFilteredGroupingPolicy(0, subject, "", domainStr)
// 	if err != nil {
// 		return nil, err
// 	}
// 	rules := make([]GroupingRule, 0, len(groupings))

// 	for _, g := range groupings {
// 		if len(g) >= 3 {
// 			rules = append(rules, GroupingRule{
// 				Sub:  g[0],
// 				Role: g[1],
// 				Dom:  g[2],
// 			})
// 		}
// 	}

// 	return rules, nil
// }

// LoadPolicy 重新加载策略（用于缓存刷新）
func (c *CasbinAdapter) LoadPolicy(ctx context.Context) error {
	started := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	_ = c.enforcer.InvalidateCache()
	err := c.enforcer.LoadPolicy()
	c.lastReloadAt = time.Now()
	c.lastReloadErr = err
	duration := time.Since(started)
	if err != nil {
		log.ErrorContext(ctx, "authz runtime policy reload failed",
			log.String("result", "failed"),
			log.Int64("duration_ms", duration.Milliseconds()),
			log.Err(err),
		)
	} else {
		log.InfoContext(ctx, "authz runtime policy reloaded",
			log.String("result", "success"),
			log.Int64("duration_ms", duration.Milliseconds()),
		)
	}
	return err
}

// AuthorizeRoute 执行路由级授权判定；路由守卫只使用租户内 all:* 范围。
func (c *CasbinAdapter) AuthorizeRoute(ctx context.Context, sub, tenantID, resourceKey, action string) (bool, error) {
	_ = ctx
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.enforcer.Enforce(sub, tenantID, resourceKey, action, defaultScopeKey)
}

// Check implements the application authorization decision port using Casbin.
func (c *CasbinAdapter) Check(ctx context.Context, request authzDomain.AuthorizationRequest) (authzDomain.AuthorizationDecision, error) {
	fact := RequestFromAuthorizationRequest(request)
	allowed, matched, err := c.enforceFact(ctx, fact)
	if err != nil {
		return authzDomain.AuthorizationDecision{}, err
	}
	if !allowed {
		return decision.Deny(time.Now()), nil
	}
	permission, err := PermissionFromPolicyRule(matched)
	if err != nil {
		return authzDomain.AuthorizationDecision{}, err
	}
	return decision.Allow(&permission, time.Now()), nil
}

func (c *CasbinAdapter) enforceFact(ctx context.Context, fact Request) (bool, PolicyRule, error) {
	_ = ctx
	c.mu.RLock()
	defer c.mu.RUnlock()

	allowed, matched, err := c.enforcer.EnforceEx(fact.Sub, fact.Dom, fact.Obj, fact.Act, normalizeScopeKey(fact.Scope))
	if err != nil {
		return false, PolicyRule{}, err
	}
	if !allowed || len(matched) < 4 {
		return allowed, PolicyRule{}, nil
	}
	return allowed, PolicyRule{
		Sub:   matched[0],
		Dom:   matched[1],
		Obj:   matched[2],
		Act:   matched[3],
		Scope: policyScopeAt(matched, 4),
	}, nil
}

// DirectRoleKeys 返回主体在指定租户域下的直接角色键。
func (c *CasbinAdapter) DirectRoleKeys(ctx context.Context, subject, tenantID string) ([]string, error) {
	_ = ctx
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.enforcer.GetRolesForUser(subject, tenantID)
}

func (c *CasbinAdapter) implicitRolesForUser(ctx context.Context, user, domain string) ([]string, error) {
	_ = ctx
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.enforcer.GetImplicitRolesForUser(user, domain)
}

// RoleNamesForSubject returns business role names for a subject inside a tenant.
func (c *CasbinAdapter) RoleNamesForSubject(ctx context.Context, subject authzDomain.Subject, tenantID string) ([]string, error) {
	roleKeys, err := c.implicitRolesForUser(ctx, SubjectKey(subject), tenantID)
	if err != nil {
		return nil, err
	}
	roleNames := make([]string, 0, len(roleKeys))
	for _, roleKey := range roleKeys {
		roleNames = append(roleNames, RoleNameFromKey(roleKey))
	}
	return roleNames, nil
}

func (c *CasbinAdapter) implicitPermissionsForUser(ctx context.Context, user, dom string) ([]PolicyRule, error) {
	_ = ctx
	c.mu.RLock()
	defer c.mu.RUnlock()

	permissions, err := c.enforcer.GetImplicitPermissionsForUser(user, dom)
	if err != nil {
		return nil, err
	}

	rules := make([]PolicyRule, 0, len(permissions))
	for _, permission := range permissions {
		if len(permission) < 4 {
			continue
		}
		rules = append(rules, PolicyRule{
			Sub:   permission[0],
			Dom:   permission[1],
			Obj:   permission[2],
			Act:   permission[3],
			Scope: policyScopeAt(permission, 4),
		})
	}
	return rules, nil
}

// PermissionsForSubject returns business permissions for a subject inside a tenant.
func (c *CasbinAdapter) PermissionsForSubject(ctx context.Context, subject authzDomain.Subject, tenantID string) ([]authzDomain.Permission, error) {
	rules, err := c.implicitPermissionsForUser(ctx, SubjectKey(subject), tenantID)
	if err != nil {
		return nil, err
	}
	permissions := make([]authzDomain.Permission, 0, len(rules))
	for _, rule := range rules {
		permission, err := PermissionFromPolicyRule(rule)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}
	return permissions, nil
}

// PermissionsForRole returns business permissions granted to a role inside a tenant.
func (c *CasbinAdapter) PermissionsForRole(ctx context.Context, roleName, tenantID string) ([]authzDomain.Permission, error) {
	rules, err := c.policyFactsForRole(ctx, roleName, tenantID)
	if err != nil {
		return nil, err
	}
	permissions := make([]authzDomain.Permission, 0, len(rules))
	for _, rule := range rules {
		permission, err := PermissionFromPolicyRule(rule)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}
	return permissions, nil
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

func policyScopeAt(values []string, idx int) string {
	if len(values) <= idx {
		return defaultScopeKey
	}
	return normalizeScopeKey(values[idx])
}

func normalizeScopeKey(scope string) string {
	normalized := ScopeFromKey(scope)
	return ScopeKey(normalized)
}

func scopeMatchFunc(args ...interface{}) (interface{}, error) {
	if len(args) != 2 {
		return false, nil
	}
	requestScope, _ := args[0].(string)
	policyScope, _ := args[1].(string)
	return scopeMatch(requestScope, policyScope), nil
}

func scopeMatch(requestScope, policyScope string) bool {
	requestScope = normalizeScopeKey(requestScope)
	policyScope = normalizeScopeKey(policyScope)
	if policyScope == defaultScopeKey {
		return true
	}
	return requestScope == policyScope
}

func resourceMatchFunc(args ...interface{}) (interface{}, error) {
	if len(args) != 2 {
		return false, nil
	}
	requestResource, _ := args[0].(string)
	policyResource, _ := args[1].(string)
	return resourceMatch(requestResource, policyResource), nil
}

func resourceMatch(requestResource, policyResource string) bool {
	requestParts := strings.Split(strings.TrimSpace(requestResource), ":")
	policyParts := strings.Split(strings.TrimSpace(policyResource), ":")
	if len(requestParts) != 4 || len(policyParts) != 4 {
		return false
	}
	for i := range requestParts {
		if policyParts[i] == "*" {
			continue
		}
		if requestParts[i] != policyParts[i] {
			return false
		}
	}
	return true
}

func actionMatchFunc(args ...interface{}) (interface{}, error) {
	if len(args) != 2 {
		return false, nil
	}
	requestAction, _ := args[0].(string)
	policyAction, _ := args[1].(string)
	return actionMatch(requestAction, policyAction), nil
}

func actionMatch(requestAction, policyAction string) bool {
	requestAction = strings.TrimSpace(requestAction)
	policyAction = strings.TrimSpace(policyAction)
	if requestAction == "" || policyAction == "" {
		return false
	}
	if requestAction == policyAction {
		return true
	}
	matched, err := regexp.MatchString("^(?:"+policyAction+")$", requestAction)
	if err != nil {
		return false
	}
	return matched
}
