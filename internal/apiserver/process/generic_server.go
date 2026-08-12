package process

import (
	"github.com/FangcunMount/iam/v3/internal/apiserver/config"
	genericapiserver "github.com/FangcunMount/iam/v3/internal/pkg/server"
)

// buildGenericServer 构建通用服务器
func buildGenericServer(cfg *config.Config) (*genericapiserver.GenericAPIServer, error) {
	// 构建通用配置
	genericConfig, err := buildGenericConfig(cfg)
	// 如果构建通用配置失败，则返回错误
	if err != nil {
		return nil, err
	}

	// 创建通用服务器
	genericServer, err := genericConfig.Complete().New()
	// 如果创建通用服务器失败，则返回错误
	if err != nil {
		return nil, err
	}

	// 返回通用服务器
	return genericServer, nil
}

// buildGenericConfig 构建通用配置
func buildGenericConfig(cfg *config.Config) (genericConfig *genericapiserver.Config, lastErr error) {
	// 创建通用配置
	genericConfig = genericapiserver.NewConfig()
	// 应用通用配置

	// 应用通用服务器运行选项
	if lastErr = cfg.GenericServerRunOptions.ApplyTo(genericConfig); lastErr != nil {
		// 如果应用通用服务器运行选项失败，则返回错误
		return
	}
	// 应用安全服务选项
	if lastErr = cfg.SecureServing.ApplyTo(genericConfig); lastErr != nil {
		// 如果应用安全服务选项失败，则返回错误
		return
	}
	// 应用不安全服务选项
	if lastErr = cfg.InsecureServing.ApplyTo(genericConfig); lastErr != nil {
		// 如果应用不安全服务选项失败，则返回错误
		return
	}
	// 返回通用配置
	return
}
