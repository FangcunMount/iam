package casbin

import (
	"sync"
	"time"
)

type runtimeHealthState struct {
	mu                sync.RWMutex
	lastReloadErr     error
	lastReloadAt      time.Time
	lastEventTenantID string
	lastEventVersion  int64
	lastEventAt       time.Time
}

func newRuntimeHealthState() *runtimeHealthState {
	return &runtimeHealthState{}
}

func (s *runtimeHealthState) recordReload(err error, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastReloadErr = err
	s.lastReloadAt = at
}

func (s *runtimeHealthState) reloadHealth() (bool, error, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastReloadErr == nil, s.lastReloadErr, s.lastReloadAt
}

func (s *runtimeHealthState) recordPolicyVersionEvent(tenantID string, version int64, eventAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastEventTenantID = tenantID
	s.lastEventVersion = version
	s.lastEventAt = eventAt
}

func (s *runtimeHealthState) details(now time.Time) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	reloadLag := time.Duration(0)
	if !s.lastEventAt.IsZero() && (s.lastReloadAt.IsZero() || s.lastEventAt.After(s.lastReloadAt)) {
		reloadLag = now.Sub(s.lastEventAt)
	}
	details := map[string]any{
		"last_event_tenant_id": s.lastEventTenantID,
		"last_event_version":   s.lastEventVersion,
		"reload_lag_ms":        reloadLag.Milliseconds(),
	}
	if !s.lastEventAt.IsZero() {
		details["last_event_at"] = s.lastEventAt.Format(time.RFC3339)
	}
	if !s.lastReloadAt.IsZero() {
		details["reloaded_at"] = s.lastReloadAt.Format(time.RFC3339)
	}
	return details
}
