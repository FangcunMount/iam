package container

const (
	moduleEventRuntime    = "event runtime"
	moduleIDP             = "idp module"
	moduleAuthn           = "authn module"
	moduleAuthz           = "authz module"
	moduleUser            = "user module"
	moduleSuggest         = "suggest module"
	moduleCacheGovernance = "cache governance"
)

// ModuleState describes whether a bootstrap step ran, whether the capability
// is available after bootstrap, and why it is degraded when available is false.
type ModuleState struct {
	Bootstrapped   bool
	Available      bool
	DegradedReason string
}

// ContainerState returns the aggregate container boot state.
func (c *Container) ContainerState() ModuleState {
	if c == nil {
		return ModuleState{}
	}
	state := ModuleState{
		Bootstrapped: c.initialized,
		Available:    c.initialized,
	}
	if len(c.bootstrapErrors) > 0 {
		state.DegradedReason = "one or more modules failed to initialize"
	}
	return state
}

// ModuleStates returns the known module states keyed by bootstrap step name.
func (c *Container) ModuleStates() map[string]ModuleState {
	states := make(map[string]ModuleState, len(c.bootstrapPlan()))
	for _, step := range c.bootstrapPlan() {
		states[step.name] = c.ModuleState(step.name)
	}
	return states
}

// ModuleState returns the state of a single bootstrap step.
func (c *Container) ModuleState(name string) ModuleState {
	if c == nil {
		return ModuleState{}
	}
	state := ModuleState{
		Bootstrapped: c.initialized,
		Available:    c.initialized && c.moduleAvailable(name),
	}
	if reason := c.bootstrapErrors[name]; reason != "" {
		state.DegradedReason = reason
	}
	return state
}

func (c *Container) moduleAvailable(name string) bool {
	switch name {
	case moduleEventRuntime:
		return c.eventCatalog != nil && c.eventPublisher != nil
	case moduleIDP:
		return c.IDPModule != nil
	case moduleAuthn:
		return c.AuthnModule != nil
	case moduleAuthz:
		return c.AuthzModule != nil
	case moduleUser:
		return c.UserModule != nil
	case moduleSuggest:
		return c.SuggestModule != nil && c.SuggestModule.IsInitialized()
	case moduleCacheGovernance:
		return c.CacheGovernanceService != nil
	default:
		return false
	}
}

func (c *Container) recordBootstrapFailure(name string, err error) {
	if err == nil {
		return
	}
	if c.bootstrapErrors == nil {
		c.bootstrapErrors = make(map[string]string)
	}
	c.bootstrapErrors[name] = err.Error()
}
