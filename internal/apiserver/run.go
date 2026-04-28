package apiserver

import "github.com/FangcunMount/iam/internal/apiserver/config"

// Run 运行指定的 APIServer。此函数不应退出。
func Run(cfg *config.Config) error {
	server, err := createAPIServer(cfg)
	if err != nil {
		return err
	}

	prepared, err := server.PrepareRun()
	if err != nil {
		return err
	}

	return prepared.Run()
}
