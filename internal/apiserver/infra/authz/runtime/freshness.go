package runtime

import (
	"context"
	"fmt"
	"time"
)

type VersionSource interface {
	ReadVersions(context.Context) (map[string]int64, error)
}

func (r *Runtime) freshnessAge() time.Duration {
	return r.snapshotAge(r.current.Load())
}
func (r *Runtime) snapshotAge(snapshot *Snapshot) time.Duration {
	if snapshot == nil || snapshot.verifiedAt.IsZero() {
		return r.config.MaxUnconfirmed
	}
	return r.now().Sub(snapshot.verifiedAt)
}
func (r *Runtime) freshSnapshot(snapshot *Snapshot) bool {
	age := r.snapshotAge(snapshot)
	freshnessAge.Set(age.Seconds())
	return age >= 0 && age < r.config.MaxUnconfirmed
}
func (r *Runtime) fresh() bool { return r.freshSnapshot(r.current.Load()) }

// Publication and proof are one atomic value; a reader of the old policy can
// never accidentally use the new policy's proof time.
func (r *Runtime) publish(snapshot *Snapshot, began time.Time) {
	r.health.mu.Lock()
	defer r.health.mu.Unlock()
	if previous := r.current.Load(); previous != nil {
		snapshot.verifiedAt = previous.verifiedAt
	}
	if r.coversTargets(snapshot) && began.After(snapshot.verifiedAt) {
		snapshot.verifiedAt = began
	}
	r.current.Store(snapshot)
}
func (r *Runtime) coversTargets(snapshot *Snapshot) bool {
	for tenantID, version := range r.health.targets {
		if snapshot.versions[tenantID] < version {
			return false
		}
	}
	return true
}
func (r *Runtime) confirm(snapshot *Snapshot, began time.Time) {
	r.health.mu.Lock()
	defer r.health.mu.Unlock()
	if !r.coversTargets(snapshot) || !began.After(snapshot.verifiedAt) {
		return
	}
	confirmed := *snapshot
	confirmed.verifiedAt = began
	r.current.CompareAndSwap(snapshot, &confirmed)
}

// Reconcile reads durable versions, including tenants whose notification was lost.
func (r *Runtime) Reconcile(ctx context.Context) (resultErr error) {
	defer func() {
		r.health.mu.Lock()
		r.health.lastSyncErr = resultErr
		r.health.mu.Unlock()
		freshnessAge.Set(r.freshnessAge().Seconds())
		policyVersionLag.Set(float64(r.health.versionLag(r.current.Load())))
		if resultErr != nil {
			versionCheckFailures.Inc()
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, r.config.SyncTimeout)
	defer cancel()
	began := r.now()
	source, ok := r.source.(VersionSource)
	if !ok {
		return fmt.Errorf("authorization version source unavailable")
	}
	versions, err := source.ReadVersions(ctx)
	if err != nil {
		return err
	}
	r.health.mu.Lock()
	if r.health.targets == nil {
		r.health.targets = make(map[string]int64)
	}
	for tenantID, version := range versions {
		if version > r.health.targets[tenantID] {
			r.health.targets[tenantID] = version
		}
	}
	r.health.mu.Unlock()
	snapshot := r.current.Load()
	if snapshot == nil || r.health.versionLag(snapshot) > 0 {
		if err := r.LoadPolicy(ctx); err != nil {
			return err
		}
		if r.health.versionLag(r.current.Load()) > 0 {
			return fmt.Errorf("authorization snapshot has not reached observed versions")
		}
		return nil
	}
	for tenantID, version := range snapshot.versions {
		if versions[tenantID] < version {
			return fmt.Errorf("durable authorization version regressed for tenant %s", tenantID)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	r.confirm(snapshot, began)
	freshnessAge.Set(r.freshnessAge().Seconds())
	return nil
}

func (r *Runtime) PolicyVersionLoaded(tenantID string, version int64) bool {
	s := r.current.Load()
	return s != nil && s.versions[tenantID] >= version
}
func (r *Runtime) SyncConfig() Config { return r.config }
func (r *Runtime) RequireSync() {
	r.health.mu.Lock()
	r.health.syncRequired = true
	r.health.mu.Unlock()
}
func (r *Runtime) SetSyncState(running, subscribed bool) {
	r.health.mu.Lock()
	r.health.syncRunning = running
	r.health.subscribed = subscribed
	r.health.mu.Unlock()
	value := float64(0)
	if subscribed {
		value = 1
	}
	subscriptionRegistered.Set(value)
}
func (r *Runtime) syncReady() bool {
	r.health.mu.RLock()
	defer r.health.mu.RUnlock()
	return !r.health.syncRequired || (r.health.syncRunning && r.health.subscribed)
}
