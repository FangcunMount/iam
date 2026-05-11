package container

import "context"

type RotationScheduler interface {
	Start(context.Context) error
	Stop() error
	IsRunning() bool
}

type OutboxRelay interface {
	DispatchDue(context.Context) error
}

type AuthzPolicySyncSubscriber interface {
	Start(context.Context) error
	Stop() error
}

type RuntimeDeps struct {
	RotationScheduler RotationScheduler
	OutboxRelay       OutboxRelay
	AuthzPolicySync   AuthzPolicySyncSubscriber
	SuggestCleanup    func() error
}

// BuildRuntimeDeps exposes background runtime collaborators without leaking
// concrete module fields into process bootstrap code.
func (c *Container) BuildRuntimeDeps() RuntimeDeps {
	if c == nil {
		return RuntimeDeps{}
	}
	return c.runtimeHooks()
}

func (c *Container) runtimeHooks() RuntimeDeps {
	var deps RuntimeDeps
	if c.AuthnModule != nil {
		deps.RotationScheduler = c.AuthnModule.RuntimeCapabilities().RotationScheduler
	}
	deps.OutboxRelay = c.OutboxRelay()
	if c.AuthzModule != nil && c.eventBus != nil {
		deps.AuthzPolicySync = c.AuthzModule.PolicySyncSubscriber(c.eventBus.Subscriber())
	}
	if c.SuggestModule != nil {
		deps.SuggestCleanup = c.SuggestModule.RuntimeCapabilities().Cleanup
	}
	return deps
}

// CriticalModulesMissing returns the startup-critical modules that failed to initialize.
func (c *Container) CriticalModulesMissing() []string {
	if c == nil {
		return []string{"container"}
	}
	missing := make([]string, 0, 4)
	if c.IDPModule == nil {
		missing = append(missing, "idp")
	}
	if c.AuthnModule == nil {
		missing = append(missing, "authn")
	}
	if c.AuthzModule == nil {
		missing = append(missing, "authz")
	}
	if c.UserModule == nil {
		missing = append(missing, "user")
	}
	return missing
}
