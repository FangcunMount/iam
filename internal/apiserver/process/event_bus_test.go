package process

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	apiserverconfig "github.com/FangcunMount/iam/v5/internal/apiserver/config"
	apiserveroptions "github.com/FangcunMount/iam/v5/internal/apiserver/options"
	genericoptions "github.com/FangcunMount/iam/v5/internal/pkg/options"
)

func TestPrepareMessagingRejectsRetiredAssemblyBeforeProvisioning(t *testing.T) {
	var requests atomic.Int64
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(http.StatusOK) }))
	defer endpoint.Close()
	catalog := writeEventCatalog(t, `
version: "1"
topics:
  authz_version:
    name: iam.authz.version.v2
events:
  iam.authz.version_changed.v2:
    topic: authz_version
    delivery: durable_outbox
    handler: iam-policy-sync
`)
	for _, provider := range []string{"", "nsq", "rocketmq"} {
		t.Run(provider, func(t *testing.T) {
			opts := apiserveroptions.NewOptions()
			opts.Events.CatalogPath = catalog
			opts.Events.ReliableMessaging.Enabled = true
			opts.NSQOptions = &genericoptions.NSQOptions{Enabled: true, ConsumerSDKEnabled: true, NSQdHTTPAddrs: []string{endpoint.URL}}
			opts.Messaging = apiserveroptions.MessagingOptions{Provider: provider, Endpoint: "127.0.0.1:1", CredentialsFile: "absent", CAFile: "absent", RoutesFile: "absent", InstanceID: "fixture"}
			server := &apiServer{cfg: &apiserverconfig.Config{Options: opts}}
			if err := server.prepareMessaging(); err == nil || !strings.Contains(err.Error(), "final RocketMQ transaction owner required") {
				t.Fatalf("retired assembly was not rejected before dependencies: %v", err)
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatalf("retired topic provisioner made %d requests", requests.Load())
	}
}

func writeEventCatalog(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "events.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Transaction mode must retain fatal startup/shutdown handling without enabling
// the retired Outbox or configuring an NSQ producer/subscriber.
func TestPrepareTransactionMessagingIndependentOfLegacyFlags(t *testing.T) {
	opts := apiserveroptions.NewOptions()
	opts.Events.ReliableMessaging.Enabled = false
	opts.NSQOptions = nil
	opts.Messaging = apiserveroptions.MessagingOptions{Provider: "rocketmq", Endpoint: "127.0.0.1:1", CredentialsFile: "explicit-credentials", CAFile: "explicit-ca", RoutesFile: "explicit-routes", InstanceID: "fixture", Transactions: map[string]apiserveroptions.TransactionOptions{"iam.mysql": {StoreID: "primary", WriterEpoch: "m3-1", RouteRevision: "m3-1"}}}
	server := &apiServer{cfg: &apiserverconfig.Config{Options: opts}}
	if !server.reliableMessagingEnabled() {
		t.Fatal("transaction runtime lost fatal startup/shutdown handling")
	}
	if err := server.prepareMessaging(); err != nil {
		t.Fatal(err)
	}
	opts.Events.ReliableMessaging.Enabled = true
	if err := server.prepareMessaging(); err == nil {
		t.Fatal("transaction owner accepted retired Relay flag")
	}
	opts.Events.ReliableMessaging.Enabled = false
	opts.Messaging.Provider = "nsq"
	if err := server.prepareMessaging(); err == nil {
		t.Fatal("transaction configuration fell through to NSQ")
	}
	var absent *apiServer
	if absent.reliableMessagingEnabled() {
		t.Fatal("nil server enabled messaging")
	}
	if err := absent.prepareMessaging(); err == nil {
		t.Fatal("nil server accepted absent configuration")
	}
}
