package apiserver

import (
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v2/internal/apiserver/config"
	"github.com/FangcunMount/iam/v2/internal/apiserver/options"
	"github.com/FangcunMount/iam/v2/pkg/app"
)

// commandDesc 命令描述
const commandDesc = `The iam contracts API server provides a clean architecture foundation
for building web applications. It includes user management, authentication,
and a modular design based on hexagonal architecture principles.`

// NewApp 创建 App
func NewApp(basename string) *app.App {
	opts := options.NewOptions()
	application := app.NewApp("iam contracts API Server",
		basename,
		app.WithDescription(commandDesc),
		app.WithDefaultValidArgs(),
		app.WithOptions(opts),
		app.WithRunFunc(run(opts)),
	)

	return application
}

func run(opts *options.Options) app.RunFunc {
	return func(basename string) error {
		// 初始化日志（使用从配置文件加载的配置）
		log.Init(opts.Log)
		defer log.Flush()

		log.Info("Starting iam ...")

		// 打印配置信息
		log.Infof("Server mode: %s", opts.GenericServerRunOptions.Mode)
		log.Infof("Health check enabled: %v", opts.GenericServerRunOptions.Healthz)

		// 根据 options 创建 app 配置
		cfg, err := config.CreateConfigFromOptions(opts)
		if err != nil {
			return err
		}

		// 运行 app
		return Run(cfg)
	}
}
