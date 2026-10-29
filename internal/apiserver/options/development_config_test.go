package options

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestDevelopmentConfigurationSelectsFinalTransactionRuntime(t *testing.T) {
	reader := viper.New()
	reader.SetConfigFile(filepath.Join("..", "..", "..", "configs", "apiserver.dev.yaml"))
	if err := reader.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	opts := NewOptions()
	if err := opts.DecodeConfig(reader); err != nil {
		t.Fatal(err)
	}
	if err := opts.ValidateTransactionStartupPolicy(); err != nil {
		t.Fatal(err)
	}
	if err := opts.Messaging.Validate(); err != nil {
		t.Fatal(err)
	}
	if opts.Messaging.SelectedProvider() != "rocketmq" || len(opts.Messaging.Transactions) != 1 {
		t.Fatal("development must use the final IAM owner")
	}
	if _, ok := opts.Messaging.Transactions["iam.mysql"]; !ok || opts.Events.ReliableMessaging.Enabled {
		t.Fatal("development must use iam.mysql without publishing Outbox")
	}
}
