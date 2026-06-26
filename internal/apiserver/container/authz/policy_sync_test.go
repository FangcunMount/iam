package authz

import (
	"context"
	"testing"
	"time"

	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/policysync"
	"github.com/FangcunMount/iam/v2/internal/apiserver/eventing"
	"github.com/stretchr/testify/require"
)

func TestAuthzPolicySyncSubscriberRegistersAndReloadsRuntime(t *testing.T) {
	t.Parallel()

	reloader := &policySyncReloaderStub{}
	recorder := &policySyncRuntimeHealthStub{}
	subscriber := &policySyncSubscriberStub{}
	module := &AuthzModule{
		policyReloader: reloader,
		runtimeHealth:  recorder,
	}

	sync := module.PolicySyncSubscriber(subscriber)
	require.NotNil(t, sync)
	require.NoError(t, sync.Start(context.Background()))

	require.Equal(t, policysync.Topic, subscriber.topic)
	require.Equal(t, sync.Channel(), subscriber.channel)
	require.Contains(t, subscriber.channel, policysync.ChannelPrefix+".")
	require.Contains(t, subscriber.channel, "#ephemeral")
	require.Equal(t, subscriber.channel, recorder.policySyncChannel)
	msg := cbmessaging.NewMessage("msg-1", []byte(`{"tenant_id":"tenant-a","version":12}`))
	msg.Metadata = map[string]string{"event_type": eventing.AuthzVersionChanged}
	require.NoError(t, subscriber.handler(context.Background(), msg))
	require.Equal(t, 1, reloader.reloads)
	require.Equal(t, "tenant-a", recorder.tenantID)
	require.Equal(t, int64(12), recorder.version)
	require.False(t, recorder.eventAt.IsZero())

	require.NoError(t, sync.Stop())
	require.True(t, subscriber.stopped)
}

type policySyncSubscriberStub struct {
	topic   string
	channel string
	handler cbmessaging.Handler
	stopped bool
}

func (s *policySyncSubscriberStub) Subscribe(topic, channel string, handler cbmessaging.Handler) error {
	s.topic = topic
	s.channel = channel
	s.handler = handler
	return nil
}

func (s *policySyncSubscriberStub) SubscribeWithMiddleware(topic, channel string, handler cbmessaging.Handler, _ ...cbmessaging.Middleware) error {
	return s.Subscribe(topic, channel, handler)
}

func (s *policySyncSubscriberStub) Stop() {
	s.stopped = true
}

func (s *policySyncSubscriberStub) Close() error {
	return nil
}

type policySyncReloaderStub struct {
	reloads int
}

func (s *policySyncReloaderStub) LoadPolicy(context.Context) error {
	s.reloads++
	return nil
}

type policySyncRuntimeHealthStub struct {
	tenantID          string
	version           int64
	eventAt           time.Time
	policySyncChannel string
}

func (s *policySyncRuntimeHealthStub) ReloadHealth() (bool, error, time.Time) {
	return true, nil, time.Time{}
}

func (s *policySyncRuntimeHealthStub) RuntimeHealthDetails() map[string]any {
	return nil
}

func (s *policySyncRuntimeHealthStub) RecordPolicyVersionEvent(tenantID string, version int64, eventAt time.Time) {
	s.tenantID = tenantID
	s.version = version
	s.eventAt = eventAt
}

func (s *policySyncRuntimeHealthStub) SetPolicySyncChannel(channel string) {
	s.policySyncChannel = channel
}
