package process

import apiserverconfig "github.com/FangcunMount/iam/internal/apiserver/config"

// Run owns the IAM apiserver process lifecycle.
func Run(cfg *apiserverconfig.Config) error {
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
