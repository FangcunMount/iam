//go:build reliable_messaging

package authz

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FangcunMount/component-base/pkg/messaging"
	oldnsq "github.com/FangcunMount/component-base/pkg/messaging/nsq"
	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policypublication"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/authorization"
	authzruntime "github.com/FangcunMount/iam/v5/internal/apiserver/infra/authz/runtime"
	sdknsq "github.com/FangcunMount/reliable-messaging/transport/nsq"
	"github.com/FangcunMount/reliable-messaging/wire/legacy"
	"github.com/nsqio/go-nsq"
	"github.com/stretchr/testify/require"
)

// The current IAM old image uses component-base v0.6.1. Unlike v0.6.11 it
// cannot hand a failed message to a stable failure channel. A new IAM instance
// must recover the authorization state from its version source instead.
// This test exercises the released old Subscriber and the IAM SDK wiring in
// one process against real NSQ with a controllable version-source stub; it is
// not a real MySQL commit, OS restart, or production proof.
func TestIAMV061CutoffThenSDKReconcilesVersionSource(t *testing.T) {
	if os.Getenv("RM_IAM_M6_REQUIRED") != "1" {
		t.Skip("disposable NSQ required")
	}
	tcp, httpURL := os.Getenv("RM_IAM_NSQ_TCP"), os.Getenv("RM_IAM_NSQ_HTTP")
	require.NotEmpty(t, tcp)
	require.NotEmpty(t, httpURL)
	host, portText, err := net.SplitHostPort(tcp)
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)
	lookup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lookup" || r.URL.Query().Get("topic") != policypublication.Topic {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-NSQ-Content-Type", "nsq; version=1.0")
		_ = json.NewEncoder(w).Encode(map[string]any{"producers": []map[string]any{{"broadcast_address": host, "tcp_port": port}}})
	}))
	defer lookup.Close()

	oldChannel := InstanceChannel("v061-cutover", int(time.Now().UnixNano()%1000000000))
	config := nsq.NewConfig()
	config.MaxAttempts = 1 // shorten the released driver's cutoff, without changing its behavior
	config.MaxInFlight = 1
	config.DefaultRequeueDelay = 20 * time.Millisecond
	config.MaxRequeueDelay = 20 * time.Millisecond
	old, err := oldnsq.NewSubscriber([]string{lookup.URL}, config)
	require.NoError(t, err)
	defer old.Close()
	var oldCalls atomic.Int32
	require.NoError(t, old.Subscribe(policypublication.Topic, oldChannel, func(context.Context, *messaging.Message) error {
		oldCalls.Add(1)
		return errors.New("controlled old policy reload failure")
	}))
	require.Eventually(t, func() bool {
		return nsqChannelClients(httpURL, policypublication.Topic, oldChannel) > 0
	}, 10*time.Second, 20*time.Millisecond, "old channel must be attached before publish")
	publisher, err := oldnsq.NewPublisher(tcp, nil)
	require.NoError(t, err)
	defer publisher.Close()
	message := messaging.NewMessage("iam-v061-cutover-event", []byte(`{"version":2}`))
	message.Metadata["event_type"] = "iam.authz.version_changed.v2"
	require.NoError(t, publisher.PublishMessage(context.Background(), policypublication.Topic, message))
	require.Eventually(t, func() bool {
		provider, ok := old.(interface{ Stats() map[string]interface{} })
		if !ok {
			return false
		}
		stats, ok := provider.Stats()["consumer_0"].(map[string]interface{})
		if !ok {
			return false
		}
		finished, _ := stats["finished"].(uint64)
		requeued, _ := stats["requeued"].(uint64)
		return finished == 1 && requeued >= 1
	}, 10*time.Second, 20*time.Millisecond, "old driver should FIN after its bounded retry")
	require.EqualValues(t, 1, oldCalls.Load(), "second delivery must skip the business handler")
	require.NoError(t, old.Close())
	require.Eventually(t, func() bool {
		return nsqChannelClients(httpURL, policypublication.Topic, oldChannel) == -1
	}, 10*time.Second, 20*time.Millisecond, "old ephemeral channel should disappear")

	source := &lifecycleSource{entered: make(chan struct{})}
	source.version.Store(1)
	runtime, err := authzruntime.NewRuntime(context.Background(), source, authorization.NewEvaluator(),
		authzruntime.WithConfig(authzruntime.Config{CheckInterval: 20 * time.Millisecond, SyncTimeout: time.Second, MaxUnconfirmed: 2 * time.Second}))
	require.NoError(t, err)
	runtime.RequireSync()
	source.version.Store(2) // changed after the new instance loaded, with no usable old notification
	driverConfig := nsq.NewConfig()
	driverConfig.DialTimeout = time.Second
	sdkSubscriber, err := sdknsq.NewSubscriber(sdknsq.SubscriberConfig{
		NSQDAddresses: []string{tcp}, Driver: driverConfig, MaxInFlight: 1, MaxAttempts: 1,
		FailedHandoffGroup: ChannelPrefix,
	})
	require.NoError(t, err)
	provisioner, err := sdknsq.NewProvisioner(&http.Client{Timeout: 5 * time.Second}, []string{httpURL})
	require.NoError(t, err)
	var newFailures atomic.Int32
	module := &AuthzModule{policyReloader: runtime, runtimeHealth: runtime}
	syncer := module.SDKPolicySyncSubscriber(sdkSubscriber, provisioner, func(context.Context, legacy.FailedHandoff) error {
		newFailures.Add(1)
		return nil
	})
	require.NotNil(t, syncer)
	syncer.channel = InstanceChannel("sdk-cutover", int(time.Now().UnixNano()%1000000000))
	require.NoError(t, syncer.Start(context.Background()))
	defer syncer.Stop()
	require.True(t, syncer.registered)
	require.True(t, runtime.PolicyVersionLoaded(2), "startup reconciliation must recover the current source version")
	require.Zero(t, newFailures.Load(), "no old stable handoff exists to audit")
}

func nsqChannelClients(base, topic, channel string) int {
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get(strings.TrimRight(base, "/") + "/stats?format=json")
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0
	}
	var state struct {
		Topics []struct {
			Name     string `json:"topic_name"`
			Channels []struct {
				Name    string            `json:"channel_name"`
				Clients []json.RawMessage `json:"clients"`
			} `json:"channels"`
		} `json:"topics"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return 0
	}
	for _, candidate := range state.Topics {
		if candidate.Name == topic {
			for _, current := range candidate.Channels {
				if current.Name == channel {
					return len(current.Clients)
				}
			}
		}
	}
	return -1
}
