package cd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	options "github.com/FangcunMount/iam/v5/internal/apiserver/options"
	"github.com/FangcunMount/iam/v5/pkg/app"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/spf13/viper"
)

func TestPreparedTransactionProfileUsesActualIAMDecoder(t *testing.T) {
	tmp := t.TempDir()
	pkg := filepath.Join(tmp, "package")
	out, err := runPreparePackage(t, pkg, filepath.Join(tmp, "package.tar.gz"), []string{
		"SEED_MOCK_AUTH_ENABLED=false", "IAM_MESSAGING_PROVIDER=rocketmq",
		"ROCKETMQ_ENDPOINT=broker.invalid:8081", "ROCKETMQ_INSTANCE_ID=iam-fixture",
		"ROCKETMQ_STORE_ID=Original-$Primary", "ROCKETMQ_WRITER_EPOCH=Original-Writer-Epoch",
		"ROCKETMQ_BINDING_REVISION=m4-iam-fixture", "ROCKETMQ_PUBLISH_ENABLED=false",
	})
	if err != nil {
		t.Fatalf("package: %v %s", err, out)
	}
	reader := viper.New()
	reader.SetConfigFile(filepath.Join(pkg, "configs/apiserver.prod.yaml"))
	if err := reader.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	opts := options.NewOptions()
	if err := app.DecodeOptions(reader, opts); err != nil {
		t.Fatal(err)
	}
	if opts.Messaging.SelectedProvider() != "rocketmq" || opts.Messaging.PublishEnabled || len(opts.Messaging.Transactions) != 1 {
		t.Fatal("final owner or closed admission missing")
	}
	b, ok := opts.Messaging.Transactions["iam.mysql"]
	if !ok || b.StoreID != "Original-$Primary" || b.WriterEpoch != "Original-Writer-Epoch" || b.RouteRevision != "m3-1" {
		t.Fatal("original owner identity changed")
	}
	limits, err := b.ProtocolLimits("iam.mysql")
	if err != nil || limits.Sends != 8 || limits.Branches != 16 || limits.CheckWorkers != 4 || limits.CheckQueue != 32 || limits.ReconcileBatch != 100 || limits.CheckTimeout != 2*time.Second || limits.RPCTimeout != 5*time.Second || limits.Lease != 60*time.Second || limits.ReconcileInterval != 5*time.Second {
		t.Fatalf("frozen budget changed: %v", err)
	}
	if opts.MigrationOptions.Enabled || opts.Events.ReliableMessaging.Enabled || opts.Events.ReliableMessaging.ShutdownTimeout != 20*time.Second {
		t.Fatal("automatic DDL, Outbox, or shutdown contract changed")
	}
	if opts.Events.CatalogPath != "/app/configs/events-m3.yaml" {
		t.Fatal("legacy event catalog selected")
	}
	catalog, err := eventcatalog.Load(filepath.Join(pkg, "configs/events-m3.yaml"))
	if err != nil || catalog.Version != "m3-1" {
		t.Fatalf("catalog: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(pkg, "configs/env/config.prod.env"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "NSQ_") || strings.Contains(string(body), "MESSAGING_TRANSACTIONS=") {
		t.Fatal("legacy endpoint or unproven map environment injected")
	}
	if !strings.Contains(string(body), "ROCKETMQ_BINDING_REVISION=m4-iam-fixture\n") {
		t.Fatal("immutable mount revision absent")
	}
}

func TestRetiredIAMProductionSelectionFailsBeforeRemovingPackage(t *testing.T) {
	for _, env := range [][]string{
		{"IAM_MESSAGING_PROVIDER=nsq"},
		{"ROCKETMQ_STORE_ID="},
		{"ROCKETMQ_WRITER_EPOCH="},
		{"ROCKETMQ_STORE_ID=" + strings.Repeat("界", 43)},
		{"ROCKETMQ_STORE_ID= \t"},
		{"ROCKETMQ_ENDPOINT=broker.invalid:65536"},
		{"ROCKETMQ_PUBLISH_ENABLED=yes"},
		{"ROCKETMQ_BINDING_REVISION=../bad"},
	} {
		tmp := t.TempDir()
		pkg := filepath.Join(tmp, "keep")
		if err := os.Mkdir(pkg, 0700); err != nil {
			t.Fatal(err)
		}
		sentinel := filepath.Join(pkg, "sentinel")
		if err := os.WriteFile(sentinel, []byte("retain"), 0600); err != nil {
			t.Fatal(err)
		}
		input := append([]string{"SEED_MOCK_AUTH_ENABLED=false"}, env...)
		out, err := runPreparePackage(t, pkg, filepath.Join(tmp, "package.tar.gz"), input)
		if err == nil {
			t.Fatal("invalid production binding accepted")
		}
		if _, err := os.Stat(sentinel); err != nil {
			t.Fatalf("invalid binding removed existing package: %v %s", err, out)
		}
	}
}
