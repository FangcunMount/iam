package container

import (
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/authn"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/authz"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/identity"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/idp"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/suggest"
	resttransport "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest"
)

// BuildRESTDeps exposes only the collaborators required by the REST transport.
func (c *Container) BuildRESTDeps(options resttransport.RouterOptions) resttransport.Deps {
	deps := resttransport.Deps{
		RouterOptions: options,
	}
	if c == nil {
		return deps
	}

	deps.CacheGovernance = c.CacheGovernanceService
	deps.Readiness = c.ReadinessChecker()
	deps.ModuleStatus.Container = toRESTModuleState(c.ContainerState())
	deps.ModuleStatus.Modules = toRESTModuleStates(c.ModuleStates())
	authn.CollectREST(c.ModuleState(moduleAuthn).Available, c.AuthnModule, &deps)
	authz.CollectREST(c.ModuleState(moduleAuthz).Available, c.AuthzModule, &deps)
	idp.CollectREST(c.ModuleState(moduleIDP).Available, c.IDPModule, &deps)
	identity.CollectREST(c.ModuleState(moduleIdentity).Available, c.IdentityModule, &deps)
	suggest.CollectREST(c.ModuleState(moduleSuggest).Available, c.SuggestModule, &deps, c.redisClient)
	return deps
}

func toRESTModuleStates(states map[string]ModuleState) map[string]resttransport.ModuleState {
	if len(states) == 0 {
		return nil
	}
	out := make(map[string]resttransport.ModuleState, len(states))
	for name, state := range states {
		out[name] = toRESTModuleState(state)
	}
	return out
}

func toRESTModuleState(state ModuleState) resttransport.ModuleState {
	return resttransport.ModuleState{
		Bootstrapped:   state.Bootstrapped,
		Available:      state.Available,
		DegradedReason: state.DegradedReason,
	}
}
