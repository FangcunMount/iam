package authz

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policypublication"
	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	sdktransport "github.com/FangcunMount/reliable-messaging/transport"
	"github.com/FangcunMount/reliable-messaging/wire/legacy"
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

	sync := module.newSDKPolicySyncSubscriber(subscriber, policyProvisionerStub{}, ignorePolicyFailure)
	require.NotNil(t, sync)
	require.NoError(t, sync.Start(context.Background()))

	require.Equal(t, policypublication.Topic, subscriber.topic)
	require.Equal(t, sync.Channel(), subscriber.channel)
	require.Contains(t, subscriber.channel, ChannelPrefix+".")
	require.Contains(t, subscriber.channel, "#ephemeral")
	require.Equal(t, subscriber.channel, recorder.policySyncChannel)
	msg := sdktransport.Received{ID: "msg-1", Payload: []byte(`{"version":12}`)}
	msg.Metadata = map[string]string{"event_type": eventing.AuthzVersionChanged}
	require.NoError(t, subscriber.handler(context.Background(), policyDeliveryStub{msg}))
	require.Equal(t, 1, reloader.reloads)
	require.Equal(t, int64(12), recorder.version)
	require.False(t, recorder.eventAt.IsZero())

	require.NoError(t, sync.Stop())
	require.True(t, subscriber.stopped)
}

type policySyncSubscriberStub struct {
	topic   string
	channel string
	handler sdktransport.Handler
	stopped bool
}

func (s *policySyncSubscriberStub) Subscribe(_ context.Context, topic, channel string, handler sdktransport.Handler, _ func(context.Context, legacy.FailedHandoff) error) error {
	s.topic = topic
	s.channel = channel
	s.handler = handler
	return nil
}

func (s *policySyncSubscriberStub) Close(context.Context) error {
	s.stopped = true
	return nil
}

type policyProvisionerStub struct{}

func (policyProvisionerStub) EnsureTopic(context.Context, string) error           { return nil }
func (policyProvisionerStub) EnsureChannel(context.Context, string, string) error { return nil }
func ignorePolicyFailure(context.Context, legacy.FailedHandoff) error             { return nil }

type policyDeliveryStub struct{ value sdktransport.Received }

func (d policyDeliveryStub) Message() sdktransport.Received { return d.value }
func (policyDeliveryStub) Ack() error                       { return nil }
func (policyDeliveryStub) Nack(error) error                 { return nil }
func (policyDeliveryStub) Settled() bool                    { return false }

type policySyncReloaderStub struct {
	reloads int
}

func (s *policySyncReloaderStub) LoadPolicy(context.Context) error {
	s.reloads++
	return nil
}

type policySyncRuntimeHealthStub struct {
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

func (s *policySyncRuntimeHealthStub) RecordPolicyVersionEvent(version int64, eventAt time.Time) {
	s.version = version
	s.eventAt = eventAt
}

func (s *policySyncRuntimeHealthStub) SetPolicySyncChannel(channel string) {
	s.policySyncChannel = channel
}
