package process

import (
	"strings"

	"github.com/FangcunMount/iam/internal/apiserver/config"
)

func degradedStartupAllowed(cfg *config.Config) bool {
	if cfg == nil || cfg.GenericServerRunOptions == nil || !cfg.GenericServerRunOptions.AllowDegradedStartup {
		return false
	}
	return !isProductionLike(runtimeMode(cfg))
}

func runtimeMode(cfg *config.Config) string {
	if cfg == nil || cfg.GenericServerRunOptions == nil {
		return "release"
	}
	return strings.ToLower(strings.TrimSpace(cfg.GenericServerRunOptions.Mode))
}

func isProductionLike(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "release", "production":
		return true
	default:
		return false
	}
}

func appModeFromServerMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "release", "production":
		return "production"
	case "test":
		return "test"
	default:
		return "development"
	}
}
