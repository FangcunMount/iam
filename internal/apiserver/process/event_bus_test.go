package process

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FangcunMount/component-base/pkg/messaging"
	apiserverconfig "github.com/FangcunMount/iam/v3/internal/apiserver/config"
	apiserveroptions "github.com/FangcunMount/iam/v3/internal/apiserver/options"
	genericoptions "github.com/FangcunMount/iam/v3/internal/pkg/options"
)

func TestNormalizeNSQConfigAppliesRuntimeDefaults(t *testing.T) {
	cfg := &messaging.Config{}

	got := normalizeNSQConfig(cfg)

	if got != cfg {
		t.Fatal("normalizeNSQConfig returned a different config pointer")
	}
	if got.NSQ.MsgTimeout != 60*time.Second {
		t.Fatalf("MsgTimeout = %v, want 60s", got.NSQ.MsgTimeout)
	}
	if got.NSQ.RequeueDelay != 5*time.Second {
		t.Fatalf("RequeueDelay = %v, want 5s", got.NSQ.RequeueDelay)
	}
	if len(got.NSQ.LookupdAddrs) != 1 || got.NSQ.LookupdAddrs[0] != "127.0.0.1:4161" {
		t.Fatalf("LookupdAddrs = %#v, want default lookupd", got.NSQ.LookupdAddrs)
	}
	if got.NSQ.NSQdAddr != "127.0.0.1:4150" {
		t.Fatalf("NSQdAddr = %q, want default nsqd", got.NSQ.NSQdAddr)
	}
	if got.NSQ.MaxAttempts != 5 {
		t.Fatalf("MaxAttempts = %d, want 5", got.NSQ.MaxAttempts)
	}
	if got.NSQ.MaxInFlight != 200 {
		t.Fatalf("MaxInFlight = %d, want 200", got.NSQ.MaxInFlight)
	}
}

func TestNormalizeNSQConfigPreservesConfiguredValues(t *testing.T) {
	cfg := &messaging.Config{NSQ: messaging.NSQConfig{
		MsgTimeout:   11 * time.Second,
		RequeueDelay: 12 * time.Second,
		LookupdAddrs: []string{"lookupd:4161"},
		NSQdAddr:     "nsqd:4150",
		MaxAttempts:  9,
		MaxInFlight:  33,
	}}

	got := normalizeNSQConfig(cfg)

	if got.NSQ.MsgTimeout != 11*time.Second ||
		got.NSQ.RequeueDelay != 12*time.Second ||
		len(got.NSQ.LookupdAddrs) != 1 || got.NSQ.LookupdAddrs[0] != "lookupd:4161" ||
		got.NSQ.NSQdAddr != "nsqd:4150" ||
		got.NSQ.MaxAttempts != 9 ||
		got.NSQ.MaxInFlight != 33 {
		t.Fatalf("configured NSQ values were not preserved: %#v", got.NSQ)
	}
}

func TestDurableTopicNamesFromCatalogReturnsOnlyDurableTopics(t *testing.T) {
	catalogPath := writeEventCatalog(t, `
version: "1"
topics:
  authz_version:
    name: iam.authz.version
  notification_sms:
    name: iam.notify.sms
events:
  iam.authz.version_changed:
    topic: authz_version
    delivery: durable_outbox
    handler: iam-policy-sync
  iam.login_otp_sms:
    topic: notification_sms
    delivery: best_effort
    handler: sms-dispatcher
`)

	topics, err := durableTopicNamesFromCatalog(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(topics) != 1 || topics[0] != "iam.authz.version" {
		t.Fatalf("durable topics = %#v, want only iam.authz.version", topics)
	}
}

func TestEnsureDurableTopicsCreatesOnlyDurableCatalogTopics(t *testing.T) {
	catalogPath := writeEventCatalog(t, `
version: "1"
topics:
  authz_version:
    name: iam.authz.version
  notification_sms:
    name: iam.notify.sms
events:
  iam.authz.version_changed:
    topic: authz_version
    delivery: durable_outbox
    handler: iam-policy-sync
  iam.login_otp_sms:
    topic: notification_sms
    delivery: best_effort
    handler: sms-dispatcher
`)

	created := make([]string, 0, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/topic/create" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		created = append(created, r.URL.Query().Get("topic"))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	s := &apiServer{cfg: &apiserverconfig.Config{Options: &apiserveroptions.Options{
		Events:     &apiserveroptions.EventOptions{CatalogPath: catalogPath},
		NSQOptions: &genericoptions.NSQOptions{Enabled: true},
	}}}
	nsqdAddr := strings.TrimPrefix(server.URL, "http://")

	if err := s.ensureDurableTopics(nsqdAddr); err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 || created[0] != "iam.authz.version" {
		t.Fatalf("created topics = %#v, want only iam.authz.version", created)
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
