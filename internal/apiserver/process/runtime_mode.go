package process

import (
	"strings"

	"github.com/FangcunMount/iam/v2/internal/apiserver/config"
)

// degradedStartupAllowed 判断是否允许降级启动
func degradedStartupAllowed(cfg *config.Config) bool {
	if cfg == nil || cfg.GenericServerRunOptions == nil || !cfg.GenericServerRunOptions.AllowDegradedStartup {
		return false
	}
	return !isProductionLike(runtimeMode(cfg))
}

// runtimeMode 获取运行时模式
func runtimeMode(cfg *config.Config) string {
	if cfg == nil || cfg.GenericServerRunOptions == nil {
		return "release"
	}
	return strings.ToLower(strings.TrimSpace(cfg.GenericServerRunOptions.Mode))
}

// isProductionLike 判断是否为生产模式
func isProductionLike(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "release", "production":
		return true
	default:
		return false
	}
}

// appModeFromServerMode 从服务器模式获取应用模式
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
