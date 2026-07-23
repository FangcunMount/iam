package process

import (
	"github.com/FangcunMount/iam/v2/internal/apiserver/config"
	genericapiserver "github.com/FangcunMount/iam/v2/internal/pkg/server"
)

func resolveRuntimeProfile(cfg *config.Config) (genericapiserver.RuntimeProfile, error) {
	if cfg == nil || cfg.GenericServerRunOptions == nil {
		return genericapiserver.ResolveRuntimeProfile(string(genericapiserver.RuntimeModeRelease))
	}
	return cfg.GenericServerRunOptions.RuntimeProfile()
}

func degradedStartupRequested(cfg *config.Config) bool {
	return cfg != nil &&
		cfg.GenericServerRunOptions != nil &&
		cfg.GenericServerRunOptions.AllowDegradedStartup
}
