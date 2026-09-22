package messaging

import (
	"context"
	"errors"
	"testing"

	"github.com/FangcunMount/reliable-messaging/message"
	"github.com/FangcunMount/reliable-messaging/transport"
	"github.com/stretchr/testify/require"
)

type policyTransportProbe struct {
	messages []message.Message
	outcome  transport.Outcome
	drainErr error
}

func (p *policyTransportProbe) Publish(_ context.Context, m message.Message) transport.Result {
	p.messages = append(p.messages, m)
	return transport.Result{Outcome: p.outcome}
}
func (p *policyTransportProbe) Drain(context.Context) error { return p.drainErr }

func TestPolicyWirePublisherKeepsLegacyIdentityAndRawPayload(t *testing.T) {
	input := message.Input{Producer: "iam", ID: "original-id", Destination: "iam.authz.version.v2",
		EventType: "iam.authz.version_changed.v2", SchemaVersion: "v2", Scope: "scope:global",
		ContentType: "application/json", OccurredAt: "2026-09-22T17:00:00+08:00", Payload: []byte("{\"version\":2}\n")}
	intent, err := message.New(input)
	require.NoError(t, err)
	fingerprint := intent.Fingerprint()
	probe := &policyTransportProbe{}
	publisher, err := NewPolicyWirePublisher(probe)
	require.NoError(t, err)
	// Golden bytes match component-base v0.6.3 PublishMessage, including the
	// original raw payload's trailing newline and legacy metadata.source.
	want := `{"type":"component-base.messaging.message.v1","uuid":"original-id","metadata":{"aggregate_id":"2","aggregate_type":"PolicyVersion","event_type":"iam.authz.version_changed.v2","source":"iam-outbox-relay"},"payload":"eyJ2ZXJzaW9uIjoyfQo="}`
	for _, outcome := range []transport.Outcome{transport.Unknown, transport.Confirmed, transport.Rejected} {
		probe.outcome = outcome
		require.Equal(t, outcome, publisher.Publish(context.Background(), intent).Outcome)
		wire := probe.messages[len(probe.messages)-1].Input()
		require.Equal(t, want, string(wire.Payload))
		wire.Payload = input.Payload
		require.Equal(t, input, wire, "routing and identity fields must not change")
		require.Equal(t, fingerprint, intent.Fingerprint())
		require.Equal(t, input, intent.Input(), "codec must not mutate the persisted intent")
	}
	for _, invalid := range []func(*message.Input){
		func(i *message.Input) { i.Producer = "another-service" },
		func(i *message.Input) { i.EventType = "another-event" },
		func(i *message.Input) { i.Scope = "scope:tenant" },
		func(i *message.Input) { i.SchemaVersion = "v3" },
		func(i *message.Input) { i.ContentType = "application/octet-stream" },
		func(i *message.Input) { i.Payload = []byte(`{"version":0}`) },
		func(i *message.Input) { i.Payload = []byte(`not-json`) },
	} {
		changed := input
		invalid(&changed)
		unsupported, err := message.New(changed)
		require.NoError(t, err)
		require.Equal(t, transport.Rejected, publisher.Publish(context.Background(), unsupported).Outcome)
	}
	require.Equal(t, transport.Rejected, publisher.Publish(context.Background(), message.Message{}).Outcome)
	require.Len(t, probe.messages, 3, "unsupported intents must not reach the broker")
	probe.drainErr = errors.New("transport still active")
	require.ErrorIs(t, publisher.Drain(context.Background()), probe.drainErr)
	_, err = NewPolicyWirePublisher(nil)
	require.Error(t, err)
}
