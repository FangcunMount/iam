//go:build reliable_messaging

package authz

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/subject"
	authzruntime "github.com/FangcunMount/iam/v5/internal/apiserver/infra/authz/runtime"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	sdknsq "github.com/FangcunMount/reliable-messaging/transport/nsq"
	"github.com/FangcunMount/reliable-messaging/wire/legacy"
	"github.com/nsqio/go-nsq"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// The old driver and its cutoff run in a pinned historical binary. Its child
// exec uses os.Args[0], so explicitly hand it the current test binary as argv0.
// A receipt from the actual current cold-start child proves the boundary.
func TestIAMV061CutoffThenSDKReconcilesMySQLVersion(t *testing.T) {
	if os.Getenv("RM_IAM_M6_REQUIRED") != "1" {
		t.Skip("disposable NSQ required")
	}
	binary := os.Getenv("IAM_RM_LEGACY_SUBSCRIBER_BINARY")
	require.True(t, filepath.IsAbs(binary), "fixed historical subscriber binary required")
	current, err := os.Executable()
	require.NoError(t, err)
	receipt := filepath.Join(t.TempDir(), "current-sdk-child.json")
	ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, binary, "-test.run=^TestIAMV061CutoffThenSDKReconcilesMySQLVersion$", "-test.v")
	child.Args[0] = current
	child.Env = append(os.Environ(), "RM_IAM_CURRENT_SUBSCRIBER_RECEIPT="+receipt)
	output, err := child.CombinedOutput()
	require.NoErrorf(t, err, "fixed old subscriber boundary failed: %s", output)
	require.NotContains(t, string(output), "--- SKIP")
	body, err := os.ReadFile(receipt)
	require.NoError(t, err, "the current SDK child must complete against the old process database")
	var result struct {
		Binary        string
		PolicyVersion int64
		Allowed       bool
	}
	require.NoError(t, json.Unmarshal(body, &result))
	require.Equal(t, current, result.Binary)
	require.EqualValues(t, 2, result.PolicyVersion)
	require.False(t, result.Allowed)
	t.Log("fixed v0.6.1 cutoff and actual current SDK cold-start revocation verified")
}

func TestIAMV061SDKColdStartProcess(t *testing.T) {
	if os.Getenv("RM_IAM_CUTOVER_CHILD") != "1" {
		t.Skip("only run by the cutover parent process")
	}
	assertRequired := func(name string) string {
		value := os.Getenv(name)
		require.NotEmpty(t, value)
		return value
	}
	db, err := gorm.Open(gormmysql.Open(assertRequired("RM_IAM_CUTOVER_DSN")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
	runtime, err := authzruntime.NewRuntime(context.Background(), authzruntime.NewMySQLSource(db), authorization.NewEvaluator(),
		authzruntime.WithConfig(authzruntime.Config{CheckInterval: 20 * time.Millisecond, SyncTimeout: time.Second, MaxUnconfirmed: 10 * time.Second}))
	require.NoError(t, err)
	require.True(t, runtime.PolicyVersionLoaded(2))
	sub, err := subject.NewUserRef(meta.FromUint64(2))
	require.NoError(t, err)
	request, err := authorization.NewRequest(sub, assertRequired("RM_IAM_CUTOVER_RESOURCE"), "retry")
	require.NoError(t, err)
	driverConfig := nsq.NewConfig()
	driverConfig.DialTimeout = time.Second
	subscriber, err := sdknsq.NewSubscriber(sdknsq.SubscriberConfig{
		NSQDAddresses: []string{assertRequired("RM_IAM_NSQ_TCP")}, Driver: driverConfig, MaxInFlight: 1, MaxAttempts: 1,
		FailedHandoffGroup: ChannelPrefix,
	})
	require.NoError(t, err)
	provisioner, err := sdknsq.NewProvisioner(&http.Client{Timeout: 5 * time.Second}, []string{assertRequired("RM_IAM_NSQ_HTTP")})
	require.NoError(t, err)
	module := &AuthzModule{policyReloader: runtime, runtimeHealth: runtime}
	syncer := module.SDKPolicySyncSubscriber(subscriber, provisioner, func(context.Context, legacy.FailedHandoff) error {
		return errors.New("unexpected failed handoff from old v0.6.1 subscriber")
	})
	require.NotNil(t, syncer)
	syncer.channel = InstanceChannel("sdk-cold-start", os.Getpid())
	require.NoError(t, syncer.Start(context.Background()))
	defer syncer.Stop()
	require.True(t, syncer.registered)
	decision, err := runtime.Check(context.Background(), request)
	require.NoError(t, err)
	require.False(t, decision.Allowed)
	require.EqualValues(t, 2, decision.PolicyVersion)
	binary, err := os.Executable()
	require.NoError(t, err)
	receipt, err := json.Marshal(struct {
		Binary        string
		PolicyVersion int64
		Allowed       bool
	}{binary, int64(decision.PolicyVersion), decision.Allowed})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(assertRequired("RM_IAM_CURRENT_SUBSCRIBER_RECEIPT"), receipt, 0600))
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
