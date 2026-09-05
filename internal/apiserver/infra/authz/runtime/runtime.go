package runtime

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authorizationapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/objectattributeadmission"
	authorizationdomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

type Runtime struct {
	providers  objectattributeadmission.Coverage
	source     Source
	evaluator  authorizationdomain.Evaluator
	current    atomic.Pointer[Snapshot]
	reloadGate chan struct{}
	health     runtimeHealth
	config     Config
	now        func() time.Time
}

func NewRuntime(ctx context.Context, source Source, evaluator authorizationdomain.Evaluator, options ...Option) (*Runtime, error) {
	if source == nil {
		return nil, fmt.Errorf("authorization runtime source is required")
	}
	runtime := &Runtime{source: source, evaluator: evaluator, reloadGate: make(chan struct{}, 1), config: DefaultConfig(), now: time.Now}
	for _, option := range options {
		option(runtime)
	}
	if err := runtime.config.Validate(); err != nil {
		return nil, err
	}
	if runtime.now == nil {
		return nil, fmt.Errorf("authorization clock required")
	}
	if err := runtime.LoadPolicy(ctx); err != nil {
		return nil, err
	}
	return runtime, nil
}

func (r *Runtime) LoadPolicy(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, r.config.SyncTimeout)
	defer cancel()
	select {
	case r.reloadGate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-r.reloadGate }()
	started := r.now()
	measured := time.Now()
	dataset, err := r.source.Load(ctx)
	if err == nil {
		var snapshot *Snapshot
		snapshot, err = BuildSnapshot(dataset, r.now(), r.providers)
		if err == nil {
			if previous := r.current.Load(); previous != nil {
				for tenantID, version := range previous.versions {
					candidate, exists := snapshot.versions[tenantID]
					if !exists || candidate < version {
						err = fmt.Errorf("authorization policy version regressed for tenant %s", tenantID)
						break
					}
				}
			}
		}
		if err == nil {
			err = ctx.Err()
		}
		if err == nil {
			r.publish(snapshot, started)
			loadedPolicyVersion.Set(float64(maxVersion(snapshot.Versions())))
			policyVersionLag.Set(float64(r.health.versionLag(snapshot)))
		}
	}
	duration := time.Since(measured)
	result := "success"
	if err != nil {
		result = "failure"
	}
	reloads.WithLabelValues(result).Inc()
	reloadLatency.Observe(duration.Seconds())
	r.health.recordReload(err, time.Now(), duration)
	return err
}

func (r *Runtime) Check(_ context.Context, request authorizationdomain.Request) (authorizationdomain.Decision, error) {
	started := time.Now()
	snapshot := r.current.Load()
	if snapshot == nil || !r.freshSnapshot(snapshot) {
		expiredChecks.Inc()
		recordCheck("error", started)
		return authorizationdomain.Decision{}, perrors.WithCode(code.ErrAuthorizationPolicyUnavailable, "authorization runtime policy unavailable")
	}
	evaluationContext, err := snapshot.evaluationContext(request)
	if err != nil {
		recordCheck("error", started)
		return authorizationdomain.Decision{}, err
	}
	decision, err := r.evaluator.Evaluate(request, evaluationContext, time.Time{})
	result := "denied"
	if err != nil {
		result = "error"
	} else if decision.Allowed {
		result = "allowed"
	}
	recordCheck(result, started)
	return decision, err
}

func (r *Runtime) GetAuthorizationSnapshot(_ context.Context, sub subject.Ref, tenantID, appName string) (authorizationapp.SubjectSnapshot, error) {
	snapshot := r.current.Load()
	if snapshot == nil || !r.freshSnapshot(snapshot) {
		expiredChecks.Inc()
		return authorizationapp.SubjectSnapshot{}, perrors.WithCode(code.ErrAuthorizationPolicyUnavailable, "authorization runtime policy unavailable")
	}
	return snapshot.SubjectSnapshot(sub, tenantID, appName)
}

func (r *Runtime) EffectiveRoleNamesForSubject(_ context.Context, sub subject.Ref, tenantID string) ([]string, error) {
	snapshot := r.current.Load()
	if snapshot == nil || !r.freshSnapshot(snapshot) {
		expiredChecks.Inc()
		return nil, perrors.WithCode(code.ErrAuthorizationPolicyUnavailable, "authorization runtime policy unavailable")
	}
	return snapshot.effectiveRoleNamesForSubject(sub, tenantID)
}

func (r *Runtime) ReloadHealth() (bool, error, time.Time) {
	_, err, at := r.health.reloadHealth()
	return r.fresh() && r.syncReady(), err, at
}

func (r *Runtime) RuntimeHealthDetails() map[string]any {
	details := r.health.details(r.now(), r.current.Load())
	details["freshness_age_seconds"] = r.freshnessAge().Seconds()
	details["fresh"] = r.fresh()
	return details
}

func (r *Runtime) RecordPolicyVersionEvent(tenantID string, version int64, eventAt time.Time) {
	r.health.recordEvent(tenantID, version, eventAt)
	policyVersionLag.Set(float64(r.health.versionLag(r.current.Load())))
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
	lastSyncErr        error
	lastReloadErr      error
	lastReloadAt       time.Time
	lastReloadDuration time.Duration
	lastEventTenantID  string
	lastEventVersion   int64
	lastEventAt        time.Time
	channel            string
	targets            map[string]int64
	syncRequired       bool
	syncRunning        bool
	subscribed         bool
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
	if h.targets == nil {
		h.targets = make(map[string]int64)
	}
	if version > h.targets[tenantID] {
		h.targets[tenantID] = version
	}
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
	if snapshot == nil {
		return 0
	}
	var maximum int64
	for tenantID, version := range h.targets {
		if lag := version - snapshot.versions[tenantID]; lag > maximum {
			maximum = lag
		}
	}
	return maximum
}

func (h *runtimeHealth) details(now time.Time, snapshot *Snapshot) map[string]any {
	h.mu.RLock()
	defer h.mu.RUnlock()
	details := map[string]any{
		"runtime": "immutable_snapshot", "policy_sync_channel": h.channel,
		"degraded":     h.lastReloadErr != nil || h.lastSyncErr != nil,
		"sync_running": h.syncRunning, "subscription_registered": h.subscribed,
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
