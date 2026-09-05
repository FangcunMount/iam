package runtime_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policypublication"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	authzruntime "github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/runtime"
	authzfixture "github.com/FangcunMount/iam/v3/internal/apiserver/testfixtures/assessment"
	"github.com/nsqio/go-nsq"
	"github.com/stretchr/testify/require"
)

type synchronizedSource struct {
	mu   sync.Mutex
	data authzruntime.Dataset
}

func (s *synchronizedSource) Load(context.Context) (authzruntime.Dataset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data, nil
}
func (s *synchronizedSource) ReadVersions(context.Context) (map[string]int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]int64{}
	for k, v := range s.data.Versions {
		out[k] = v
	}
	return out, nil
}
func (s *synchronizedSource) revoke(version int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Grants = nil
	s.data.Versions = map[string]int64{"fangcun": version}
}
func TestRealNSQTwoRuntimesRevokeLossAndReconnect(t *testing.T) {
	address := os.Getenv("IAM_AUTHZ_TEST_NSQD")
	if address == "" {
		t.Skip("IAM_AUTHZ_TEST_NSQD is not configured")
	}
	source := &synchronizedSource{data: assessmentDataset(t)}
	config := nsq.NewConfig()
	config.DialTimeout = time.Second
	config.ReadTimeout = 5 * time.Second
	config.HeartbeatInterval = time.Second
	producer, err := nsq.NewProducer(address, config)
	require.NoError(t, err)
	producer.SetLogger(nil, nsq.LogLevelError)
	defer producer.Stop()
	require.NoError(t, producer.Ping())
	runtimes := make([]*authzruntime.Runtime, 2)
	for i := range runtimes {
		runtimes[i], err = authzruntime.NewRuntime(context.Background(), source, authorization.NewEvaluator(), authzruntime.WithAttributeProviders(authzfixture.Policy()))
		require.NoError(t, err)
	}
	topic := fmt.Sprintf("iam_authz_test_%d", time.Now().UnixNano())
	connect := func() []*nsq.Consumer {
		consumers := make([]*nsq.Consumer, 2)
		var handshakes atomic.Int32
		for i, runtime := range runtimes {
			c, err := nsq.NewConsumer(topic, fmt.Sprintf("instance%d#ephemeral", i), config)
			require.NoError(t, err)
			c.SetLogger(nil, nsq.LogLevelError)
			handler := policypublication.NewService(runtime, runtime)
			var handshake sync.Once
			t.Cleanup(c.Stop)
			c.AddHandler(nsq.HandlerFunc(func(msg *nsq.Message) error {
				if string(msg.Body) == "test-barrier" {
					handshake.Do(func() { handshakes.Add(1) })
					return nil
				}
				return handler.Handle(context.Background(), msg.Body, "")
			}))
			require.NoError(t, c.ConnectToNSQD(address))
			consumers[i] = c
		}
		// Connect may return before NSQD has processed SUB. Prove both channels
		// receive messages before testing a single version notification.
		require.Eventually(t, func() bool {
			if handshakes.Load() == 2 {
				return true
			}
			if err := producer.Publish(topic, []byte("test-barrier")); err != nil {
				t.Log(err)
			}
			return false
		}, 5*time.Second, 20*time.Millisecond)
		return consumers
	}
	disconnect := func(consumers []*nsq.Consumer) {
		for _, c := range consumers {
			c.Stop()
		}
		for _, c := range consumers {
			select {
			case <-c.StopChan:
			case <-time.After(5 * time.Second):
				t.Fatal("NSQ consumer did not stop")
			}
		}
	}
	consumers := connect()
	request := checkRequest(t, 2, "retry", "adhoc")
	for _, r := range runtimes {
		decision, err := r.Check(context.Background(), request)
		require.NoError(t, err)
		require.True(t, decision.Allowed)
	}
	source.revoke(10)
	require.NoError(t, producer.Publish(topic, []byte(`{"tenant_id":"fangcun","version":10}`)))
	require.Eventually(t, func() bool {
		return runtimes[0].PolicyVersionLoaded("fangcun", 10) && runtimes[1].PolicyVersionLoaded("fangcun", 10)
	}, 10*time.Second, 10*time.Millisecond)
	disconnect(consumers) // interrupt both subscription connections
	source.revoke(11)     // no event delivered; database polling must compensate
	for _, r := range runtimes {
		require.NoError(t, r.Reconcile(context.Background()))
		require.True(t, r.PolicyVersionLoaded("fangcun", 11))
	}
	consumers = connect()
	defer disconnect(consumers)
	source.revoke(12)
	require.NoError(t, producer.Publish(topic, []byte(`{"tenant_id":"fangcun","version":12}`)))
	require.Eventually(t, func() bool {
		return runtimes[0].PolicyVersionLoaded("fangcun", 12) && runtimes[1].PolicyVersionLoaded("fangcun", 12)
	}, 10*time.Second, 10*time.Millisecond)
	for _, r := range runtimes {
		decision, err := r.Check(context.Background(), request)
		require.NoError(t, err)
		require.False(t, decision.Allowed)
		require.EqualValues(t, 12, decision.PolicyVersion)
	}
}
