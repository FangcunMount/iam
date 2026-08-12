package container

import (
	"context"

	"github.com/FangcunMount/iam/v3/internal/apiserver/container/authn"
	"github.com/FangcunMount/iam/v3/internal/apiserver/container/authz"
	"github.com/FangcunMount/iam/v3/internal/apiserver/container/suggest"
)

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
	AuthzPolicySync   authz.PolicySyncSubscriber
	SuggestCleanup    func() error
	IdentityCleanup   func() error
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
	var rotation authn.KeyRotationScheduler
	authn.CollectRuntime(c.AuthnModule, &rotation)
	deps.RotationScheduler = rotation
	deps.OutboxRelay = c.OutboxRelay()
	if c.eventBus != nil {
		authz.CollectRuntime(c.AuthzModule, c.eventBus.Subscriber(), &deps.AuthzPolicySync)
	}
	var cleanup func() error
	suggest.CollectRuntime(c.SuggestModule, &cleanup)
	deps.SuggestCleanup = cleanup
	if c.IdentityModule != nil {
		deps.IdentityCleanup = c.IdentityModule.Cleanup
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
	if c.IdentityModule == nil {
		missing = append(missing, "identity")
	}
	return missing
}
