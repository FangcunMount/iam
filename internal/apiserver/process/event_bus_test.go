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
	apiserverconfig "github.com/FangcunMount/iam/v5/internal/apiserver/config"
	apiserveroptions "github.com/FangcunMount/iam/v5/internal/apiserver/options"
	genericoptions "github.com/FangcunMount/iam/v5/internal/pkg/options"
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
    name: iam.authz.version.v2
  notification_sms:
    name: iam.notify.sms
events:
  iam.authz.version_changed.v2:
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
	if len(topics) != 1 || topics[0] != "iam.authz.version.v2" {
		t.Fatalf("durable topics = %#v, want only iam.authz.version.v2", topics)
	}
}

func TestEnsureDurableTopicsCreatesOnlyDurableCatalogTopics(t *testing.T) {
	catalogPath := writeEventCatalog(t, `
version: "1"
topics:
  authz_version:
    name: iam.authz.version.v2
  notification_sms:
    name: iam.notify.sms
events:
  iam.authz.version_changed.v2:
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
	if len(created) != 1 || created[0] != "iam.authz.version.v2" {
		t.Fatalf("created topics = %#v, want only iam.authz.version.v2", created)
	}
}

func TestPrepareMessagingUsesSDKProvisionerWithoutLegacyBus(t *testing.T) {
	catalogPath := writeEventCatalog(t, `
version: "1"
topics:
  authz_version:
    name: iam.authz.version.v2
  notification_sms:
    name: iam.notify.sms
events:
  iam.authz.version_changed.v2:
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
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		created = append(created, r.URL.Query().Get("topic"))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	opts := apiserveroptions.NewEventOptions()
	opts.CatalogPath = catalogPath
	opts.ReliableMessaging.Enabled = true
	s := &apiServer{cfg: &apiserverconfig.Config{Options: &apiserveroptions.Options{
		Events: opts,
		NSQOptions: &genericoptions.NSQOptions{Enabled: true, ConsumerSDKEnabled: true,
			NSQdAddr: "127.0.0.1:1", NSQdHTTPAddrs: []string{server.URL}},
	}}}
	bus, err := s.prepareMessaging()
	if err != nil {
		t.Fatal(err)
	}
	if bus != nil {
		t.Fatal("SDK mode must not create a legacy EventBus")
	}
	if len(created) != 1 || created[0] != "iam.authz.version.v2" {
		t.Fatalf("created topics = %#v, want only durable authorization topic", created)
	}
}

func TestPrepareMessagingFailsClosedWhenSDKTopicPreparationFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)
	opts := apiserveroptions.NewEventOptions()
	opts.CatalogPath = writeEventCatalog(t, `
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
	opts.ReliableMessaging.Enabled = true
	s := &apiServer{cfg: &apiserverconfig.Config{Options: &apiserveroptions.Options{
		Events: opts,
		NSQOptions: &genericoptions.NSQOptions{Enabled: true, ConsumerSDKEnabled: true,
			NSQdHTTPAddrs: []string{server.URL}},
	}}}
	bus, err := s.prepareMessaging()
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("topic preparation error = %v, want HTTP 503", err)
	}
	if bus != nil {
		t.Fatal("failed SDK preparation must not fall back to legacy EventBus")
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
