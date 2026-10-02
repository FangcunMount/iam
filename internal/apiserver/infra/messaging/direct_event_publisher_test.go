package messaging

import (
	"context"
	"testing"
	"time"

	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/FangcunMount/iam/v5/pkg/eventcodec"
	"github.com/FangcunMount/reliable-messaging/transport"
	"github.com/stretchr/testify/require"
)

type directTransportProbe struct {
	topic   string
	body    []byte
	outcome transport.Outcome
	calls   int
}

func (p *directTransportProbe) PublishRaw(_ context.Context, topic string, body []byte) transport.Result {
	p.topic = topic
	p.body = append([]byte(nil), body...)
	p.calls++
	return transport.Result{Outcome: p.outcome}
}

func TestDirectEventPublisherPreservesLegacySMSWireAndOutcome(t *testing.T) {
	cfg, err := eventcatalog.Parse([]byte(`version: "1"
topics:
  sms:
    name: iam.notify.sms
  version:
    name: iam.authz.version.v2
events:
  iam.login_otp_sms:
    topic: sms
    delivery: best_effort
    aggregate: LoginOTP
    domain: authn
    handler: sms-dispatcher
  iam.authz.version_changed.v2:
    topic: version
    delivery: durable_outbox
    aggregate: PolicyVersion
    domain: authz
    handler: iam-policy-sync
`))
	require.NoError(t, err)
	probe := &directTransportProbe{}
	publisher, err := NewDirectEventPublisher(eventcatalog.NewCatalog(cfg), probe, "iam-apiserver")
	require.NoError(t, err)
	evt := event.Event[map[string]string]{
		BaseEvent: event.BaseEvent{
			ID: "otp-event-1", EventTypeValue: "iam.login_otp_sms",
			OccurredAtValue:    time.Date(2026, 9, 27, 22, 0, 0, 0, time.FixedZone("UTC+8", 8*3600)),
			AggregateTypeValue: "LoginOTP", AggregateIDValue: "+8613800138000",
		},
		Data: map[string]string{"event_type": "iam.login_otp_sms", "scene": "login", "phone_e164": "+8613800138000", "code": "123456"},
	}
	oldPayload, err := eventcodec.EncodePayload(evt)
	require.NoError(t, err)
	wantWire, err := cbmessaging.EncodeMessagePayload(&cbmessaging.Message{UUID: evt.EventID(), Payload: oldPayload, Metadata: eventcodec.MetadataFromEvent(evt, "iam-apiserver")})
	require.NoError(t, err)
	for _, tc := range []struct {
		outcome transport.Outcome
		wantErr error
	}{
		{transport.Confirmed, nil},
		{transport.Unknown, ErrDirectPublishUnknown},
		{transport.Rejected, ErrDirectPublishRejected},
	} {
		probe.outcome = tc.outcome
		err := publisher.Publish(context.Background(), evt)
		if tc.wantErr == nil {
			require.NoError(t, err)
		} else {
			require.ErrorIs(t, err, tc.wantErr)
		}
		require.Equal(t, "iam.notify.sms", probe.topic)
		require.Equal(t, wantWire, probe.body, "SDK direct publisher must preserve the old complete wire envelope")
	}
	require.Equal(t, 3, probe.calls, "outcome handling must not retry or create a new event")

	durable := evt
	durable.EventTypeValue = "iam.authz.version_changed.v2"
	require.ErrorContains(t, publisher.Publish(context.Background(), durable), "must be staged to outbox")
	unknown := evt
	unknown.EventTypeValue = "unknown"
	require.ErrorContains(t, publisher.Publish(context.Background(), unknown), "not found in catalog")
	require.Equal(t, 3, probe.calls, "invalid routes must not reach NSQ")
	require.NoError(t, publisher.Publish(context.Background(), nil))
	_, err = NewDirectEventPublisher(nil, probe, "iam")
	require.Error(t, err)
	_, err = NewDirectEventPublisher(eventcatalog.NewCatalog(cfg), nil, "iam")
	require.Error(t, err)
}
