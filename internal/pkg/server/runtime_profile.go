package server

import (
	"fmt"
	"strings"
)

// RuntimeMode is the canonical Gin server mode accepted by iam-apiserver.
type RuntimeMode string

const (
	RuntimeModeDebug   RuntimeMode = "debug"
	RuntimeModeTest    RuntimeMode = "test"
	RuntimeModeRelease RuntimeMode = "release"
)

// Environment is the application environment derived from RuntimeMode.
type Environment string

const (
	EnvironmentDevelopment Environment = "development"
	EnvironmentTest        Environment = "test"
	EnvironmentProduction  Environment = "production"
)

// RuntimeProfile is the single resolved runtime-mode truth shared by bootstrap
// and downstream modules.
type RuntimeProfile struct {
	ServerMode  RuntimeMode
	Environment Environment
}

// ResolveRuntimeProfile normalizes and validates the configured server mode.
func ResolveRuntimeProfile(raw string) (RuntimeProfile, error) {
	switch RuntimeMode(strings.ToLower(strings.TrimSpace(raw))) {
	case RuntimeModeDebug:
		return RuntimeProfile{
			ServerMode:  RuntimeModeDebug,
			Environment: EnvironmentDevelopment,
		}, nil
	case RuntimeModeTest:
		return RuntimeProfile{
			ServerMode:  RuntimeModeTest,
			Environment: EnvironmentTest,
		}, nil
	case RuntimeModeRelease:
		return RuntimeProfile{
			ServerMode:  RuntimeModeRelease,
			Environment: EnvironmentProduction,
		}, nil
	default:
		return RuntimeProfile{}, fmt.Errorf("server.mode must be one of debug, test, release")
	}
}

// IsProductionLike reports whether production safety rules must apply.
func (p RuntimeProfile) IsProductionLike() bool {
	return p.Environment == EnvironmentProduction
}

// IsDevelopment reports whether development-only fallbacks may apply.
func (p RuntimeProfile) IsDevelopment() bool {
	return p.Environment == EnvironmentDevelopment
}

// AllowsDegraded applies the production invariant to an explicit degraded
// startup request.
func (p RuntimeProfile) AllowsDegraded(requested bool) bool {
	return requested && !p.IsProductionLike()
}
