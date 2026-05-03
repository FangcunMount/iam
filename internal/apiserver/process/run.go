package process

import apiserverconfig "github.com/FangcunMount/iam/v2/internal/apiserver/config"

// Run 拥有 IAM APIServer 全部进程生命周期。
// create server -> prepare server -> run server
func Run(cfg *apiserverconfig.Config) error {
	// 创建 APIServer
	server, err := createAPIServer(cfg)
	if err != nil {
		return err
	}

	// 准备运行 APIServer
	prepared, err := server.PrepareRun()
	if err != nil {
		return err
	}

	// 运行 APIServer
	return prepared.Run()
}
