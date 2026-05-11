package casbin

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/decision"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/permission"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/subject"
)

type RuntimeAdapters struct {
	DecisionEngine        *DecisionEngine
	RouteAuthorizer       *RouteAuthorizer
	SnapshotStore         *SnapshotStore
	RolePermissionStore   *RolePermissionStore
	PolicyReloader        *PolicyReloader
	RuntimeHealthReporter *RuntimeHealthReporter
	RoleNameReader        *RoleNameReader
}

func NewRuntimeAdapters(adapter *CasbinAdapter) RuntimeAdapters {
	return RuntimeAdapters{
		DecisionEngine:        &DecisionEngine{adapter: adapter},
		RouteAuthorizer:       &RouteAuthorizer{adapter: adapter},
		SnapshotStore:         &SnapshotStore{adapter: adapter},
		RolePermissionStore:   &RolePermissionStore{adapter: adapter},
		PolicyReloader:        &PolicyReloader{adapter: adapter},
		RuntimeHealthReporter: &RuntimeHealthReporter{adapter: adapter},
		RoleNameReader:        &RoleNameReader{adapter: adapter},
	}
}

type DecisionEngine struct {
	adapter *CasbinAdapter
}

func (e *DecisionEngine) Check(ctx context.Context, request decision.Request) (decision.Decision, error) {
	return e.adapter.Check(ctx, request)
}

type RouteAuthorizer struct {
	adapter *CasbinAdapter
}

func (a *RouteAuthorizer) AuthorizeRoute(ctx context.Context, sub, tenantID, resourceKey, action string) (bool, error) {
	return a.adapter.AuthorizeRoute(ctx, sub, tenantID, resourceKey, action)
}

func (a *RouteAuthorizer) DirectRoleKeys(ctx context.Context, sub, tenantID string) ([]string, error) {
	return a.adapter.DirectRoleKeys(ctx, sub, tenantID)
}

type SnapshotStore struct {
	adapter *CasbinAdapter
}

func (s *SnapshotStore) RoleNamesForSubject(ctx context.Context, sub subject.Ref, tenantID string) ([]string, error) {
	return s.adapter.RoleNamesForSubject(ctx, sub, tenantID)
}

func (s *SnapshotStore) PermissionsForSubject(ctx context.Context, sub subject.Ref, tenantID string) ([]permission.Permission, error) {
	return s.adapter.PermissionsForSubject(ctx, sub, tenantID)
}

type RolePermissionStore struct {
	adapter *CasbinAdapter
}

func (s *RolePermissionStore) PermissionsForRole(ctx context.Context, roleName, tenantID string) ([]permission.Permission, error) {
	return s.adapter.PermissionsForRole(ctx, roleName, tenantID)
}

type PolicyReloader struct {
	adapter *CasbinAdapter
}

func (r *PolicyReloader) LoadPolicy(ctx context.Context) error {
	return r.adapter.LoadPolicy(ctx)
}

func (r *PolicyReloader) InvalidateCache() {
	r.adapter.InvalidateCache()
}

type RuntimeHealthReporter struct {
	adapter *CasbinAdapter
}

func (r *RuntimeHealthReporter) ReloadHealth() (bool, error, time.Time) {
	return r.adapter.ReloadHealth()
}

func (r *RuntimeHealthReporter) RuntimeHealthDetails() map[string]any {
	return r.adapter.RuntimeHealthDetails()
}

func (r *RuntimeHealthReporter) RecordPolicyVersionEvent(tenantID string, version int64, eventAt time.Time) {
	r.adapter.RecordPolicyVersionEvent(tenantID, version, eventAt)
}

type RoleNameReader struct {
	adapter *CasbinAdapter
}

func (r *RoleNameReader) RoleNamesForSubject(ctx context.Context, sub subject.Ref, tenantID string) ([]string, error) {
	return r.adapter.RoleNamesForSubject(ctx, sub, tenantID)
}

func (c *CasbinAdapter) RecordPolicyVersionEvent(tenantID string, version int64, eventAt time.Time) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastEventTenantID = tenantID
	c.lastEventVersion = version
	c.lastEventAt = eventAt
}

func (c *CasbinAdapter) RuntimeHealthDetails() map[string]any {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	reloadLag := time.Duration(0)
	if !c.lastEventAt.IsZero() && (c.lastReloadAt.IsZero() || c.lastEventAt.After(c.lastReloadAt)) {
		reloadLag = time.Since(c.lastEventAt)
	}
	details := map[string]any{
		"last_event_tenant_id": c.lastEventTenantID,
		"last_event_version":   c.lastEventVersion,
		"reload_lag_ms":        reloadLag.Milliseconds(),
	}
	if !c.lastEventAt.IsZero() {
		details["last_event_at"] = c.lastEventAt.Format(time.RFC3339)
	}
	if !c.lastReloadAt.IsZero() {
		details["reloaded_at"] = c.lastReloadAt.Format(time.RFC3339)
	}
	return details
}
