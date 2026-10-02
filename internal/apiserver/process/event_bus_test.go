package process

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apiserverconfig "github.com/FangcunMount/iam/v5/internal/apiserver/config"
	apiserveroptions "github.com/FangcunMount/iam/v5/internal/apiserver/options"
	genericoptions "github.com/FangcunMount/iam/v5/internal/pkg/options"
)

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
	err := s.prepareMessaging()
	if err != nil {
		t.Fatal(err)
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
	err := s.prepareMessaging()
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("topic preparation error = %v, want HTTP 503", err)
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
