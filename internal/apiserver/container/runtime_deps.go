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

type RuntimeDeps struct {
	RotationScheduler RotationScheduler
	OutboxRelay       OutboxRelay
	SuggestCleanup    func() error
}

// BuildRuntimeDeps exposes background runtime collaborators without leaking
// concrete module fields into process bootstrap code.
func (c *Container) BuildRuntimeDeps() RuntimeDeps {
	var deps RuntimeDeps
	if c == nil {
		return deps
	}
	if c.AuthnModule != nil {
		deps.RotationScheduler = c.AuthnModule.RotationScheduler
	}
	deps.OutboxRelay = c.OutboxRelay()
	if c.SuggestModule != nil {
		deps.SuggestCleanup = c.SuggestModule.Cleanup
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
