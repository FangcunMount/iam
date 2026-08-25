package native

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	authzruntime "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/runtime"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
)

type Runtime struct {
	source  Source
	current atomic.Pointer[Snapshot]
	health  runtimeHealth
}

func NewRuntime(ctx context.Context, source Source) (*Runtime, error) {
	if source == nil {
		return nil, fmt.Errorf("authorization runtime source is required")
	}
	runtime := &Runtime{source: source}
	if err := runtime.LoadPolicy(ctx); err != nil {
		return nil, err
	}
	return runtime, nil
}

func (r *Runtime) LoadPolicy(ctx context.Context) error {
	started := time.Now()
	dataset, err := r.source.Load(ctx)
	if err == nil {
		var snapshot *Snapshot
		snapshot, err = BuildSnapshot(dataset, time.Now())
		if err == nil {
			r.current.Store(snapshot)
			loadedPolicyVersion.Set(float64(maxVersion(snapshot.Versions())))
			policyVersionLag.Set(float64(r.health.versionLag(snapshot)))
		}
	}
	duration := time.Since(started)
	result := "success"
	if err != nil {
		result = "failure"
	}
	reloads.WithLabelValues(result).Inc()
	reloadLatency.Observe(duration.Seconds())
	r.health.recordReload(err, time.Now(), duration)
	return err
}

func (r *Runtime) InvalidateCache() {}

func (r *Runtime) Check(_ context.Context, request authzruntime.Request) (authzruntime.Decision, error) {
	started := time.Now()
	snapshot := r.current.Load()
	if snapshot == nil {
		recordCheck("error", started)
		return authzruntime.Decision{}, fmt.Errorf("authorization runtime snapshot is unavailable")
	}
	decision, err := snapshot.Check(request)
	result := "denied"
	if err != nil {
		result = "error"
	} else if decision.Allowed {
		result = "allowed"
	}
	recordCheck(result, started)
	return decision, err
}

func (r *Runtime) GetAuthorizationSnapshot(_ context.Context, sub subject.Ref, tenantID, appName string) (authzruntime.SubjectSnapshot, error) {
	snapshot := r.current.Load()
	if snapshot == nil {
		return authzruntime.SubjectSnapshot{}, fmt.Errorf("authorization runtime snapshot is unavailable")
	}
	return snapshot.SubjectSnapshot(sub, tenantID, appName)
}

func (r *Runtime) AuthorizeRoute(ctx context.Context, subjectKey, tenantID, resourceKey, action string) (bool, error) {
	sub, err := parseSubjectKey(subjectKey)
	if err != nil {
		return false, err
	}
	object, err := authzruntime.NewObjectContext("", nil)
	if err != nil {
		return false, err
	}
	request, err := authzruntime.NewRequest(sub, tenantID, resourceKey, action, object)
	if err != nil {
		return false, err
	}
	decision, err := r.Check(ctx, request)
	return decision.Allowed, err
}

func (r *Runtime) DirectRoleKeys(_ context.Context, subjectKey, tenantID string) ([]string, error) {
	sub, err := parseSubjectKey(subjectKey)
	if err != nil {
		return nil, err
	}
	snapshot := r.current.Load()
	if snapshot == nil {
		return nil, fmt.Errorf("authorization runtime snapshot is unavailable")
	}
	return snapshot.DirectRoleKeys(sub, tenantID)
}

func (r *Runtime) RoleNamesForSubject(_ context.Context, sub subject.Ref, tenantID string) ([]string, error) {
	snapshot := r.current.Load()
	if snapshot == nil {
		return nil, fmt.Errorf("authorization runtime snapshot is unavailable")
	}
	return snapshot.roleNamesForSubject(sub, tenantID)
}

func (r *Runtime) ReloadHealth() (bool, error, time.Time) {
	return r.health.reloadHealth()
}

func (r *Runtime) RuntimeHealthDetails() map[string]any {
	return r.health.details(time.Now(), r.current.Load())
}

func (r *Runtime) RecordPolicyVersionEvent(tenantID string, version int64, eventAt time.Time) {
	r.health.recordEvent(tenantID, version, eventAt)
	loaded := int64(0)
	if snapshot := r.current.Load(); snapshot != nil {
		loaded = snapshot.Versions()[tenantID]
	}
	lag := version - loaded
	if lag < 0 {
		lag = 0
	}
	policyVersionLag.Set(float64(lag))
}

func recordCheck(result string, started time.Time) {
	authorizationChecks.WithLabelValues(result).Inc()
	authorizationLatency.WithLabelValues(result).Observe(time.Since(started).Seconds())
}

func maxVersion(versions map[string]int64) int64 {
	var maximum int64
	for _, version := range versions {
		if version > maximum {
			maximum = version
		}
	}
	return maximum
}

func (r *Runtime) SetPolicySyncChannel(channel string) { r.health.setChannel(channel) }

type runtimeHealth struct {
	mu                 sync.RWMutex
	lastReloadErr      error
	lastReloadAt       time.Time
	lastReloadDuration time.Duration
	lastEventTenantID  string
	lastEventVersion   int64
	lastEventAt        time.Time
	channel            string
}

func (h *runtimeHealth) recordReload(err error, at time.Time, duration time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastReloadErr = err
	h.lastReloadAt = at
	h.lastReloadDuration = duration
}

func (h *runtimeHealth) reloadHealth() (bool, error, time.Time) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastReloadErr == nil, h.lastReloadErr, h.lastReloadAt
}

func (h *runtimeHealth) recordEvent(tenantID string, version int64, at time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastEventTenantID = tenantID
	h.lastEventVersion = version
	h.lastEventAt = at
}

func (h *runtimeHealth) setChannel(channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.channel = channel
}

func (h *runtimeHealth) versionLag(snapshot *Snapshot) int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if snapshot == nil || h.lastEventTenantID == "" {
		return 0
	}
	lag := h.lastEventVersion - snapshot.Versions()[h.lastEventTenantID]
	if lag < 0 {
		return 0
	}
	return lag
}

func (h *runtimeHealth) details(now time.Time, snapshot *Snapshot) map[string]any {
	h.mu.RLock()
	defer h.mu.RUnlock()
	details := map[string]any{
		"runtime": "native", "policy_sync_channel": h.channel,
		"last_event_tenant_id": h.lastEventTenantID, "last_event_version": h.lastEventVersion,
		"reload_duration_ms": h.lastReloadDuration.Milliseconds(),
	}
	if !h.lastReloadAt.IsZero() {
		details["reloaded_at"] = h.lastReloadAt.Format(time.RFC3339)
	}
	if !h.lastEventAt.IsZero() {
		details["last_event_at"] = h.lastEventAt.Format(time.RFC3339)
		if h.lastEventAt.After(h.lastReloadAt) {
			details["reload_lag_ms"] = now.Sub(h.lastEventAt).Milliseconds()
		}
	}
	if snapshot != nil {
		details["loaded_policy_versions"] = snapshot.Versions()
	}
	return details
}
