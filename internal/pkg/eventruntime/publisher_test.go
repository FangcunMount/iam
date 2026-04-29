package eventruntime

import (
	"context"
	"errors"
	"testing"
	"time"

	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/internal/apiserver/infra/sms"
	"github.com/FangcunMount/iam/internal/pkg/eventcatalog"
	"github.com/stretchr/testify/require"
)

func TestRoutingPublisherRejectsDurableOutboxEvents(t *testing.T) {
	catalog := testEventRuntimeCatalog(t)
	publisher := NewRoutingPublisher(RoutingPublisherOptions{
		Catalog:     catalog,
		MQPublisher: &eventRuntimePublisherStub{},
		Mode:        PublishModeMQ,
	})

	err := publisher.Publish(context.Background(), policy.NewVersionChangedEvent("tenant-a", 1))

	require.Error(t, err)
	require.ErrorContains(t, err, "durable_outbox")
}

func TestRoutingPublisherPublishesBestEffortEvent(t *testing.T) {
	catalog := testEventRuntimeCatalog(t)
	mq := &eventRuntimePublisherStub{}
	publisher := NewRoutingPublisher(RoutingPublisherOptions{
		Catalog:     catalog,
		MQPublisher: mq,
		Mode:        PublishModeMQ,
	})

	err := publisher.Publish(context.Background(), sms.NewLoginOTPSMSEvent("+8613800138000", "123456"))

	require.NoError(t, err)
	require.Equal(t, []string{"iam.notify.sms"}, mq.topics)
	require.Len(t, mq.messages, 1)
	require.Equal(t, eventcatalog.LoginOTPSMS, mq.messages[0].Metadata["event_type"])
}

func TestRoutingPublisherPropagatesPublishError(t *testing.T) {
	catalog := testEventRuntimeCatalog(t)
	publisher := NewRoutingPublisher(RoutingPublisherOptions{
		Catalog:     catalog,
		MQPublisher: &eventRuntimePublisherStub{err: errors.New("mq down")},
		Mode:        PublishModeMQ,
	})

	err := publisher.Publish(context.Background(), sms.NewLoginOTPSMSEvent("+8613800138000", "123456"))

	require.ErrorContains(t, err, "mq down")
}

func TestRoutingPublisherRejectsUnknownEvent(t *testing.T) {
	publisher := NewRoutingPublisher(RoutingPublisherOptions{
		Catalog:     testEventRuntimeCatalog(t),
		MQPublisher: &eventRuntimePublisherStub{},
		Mode:        PublishModeMQ,
	})

	err := publisher.Publish(context.Background(), unknownEvent{})

	require.ErrorContains(t, err, "not found in catalog")
}

func testEventRuntimeCatalog(t *testing.T) *eventcatalog.Catalog {
	t.Helper()
	cfg, err := eventcatalog.Parse([]byte(`
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
    aggregate: PolicyVersion
    domain: authz
    handler: iam-policy-sync
  iam.login_otp_sms:
    topic: notification_sms
    delivery: best_effort
    aggregate: LoginOTP
    domain: authn
    handler: sms-dispatcher
`))
	require.NoError(t, err)
	return eventcatalog.NewCatalog(cfg)
}

type eventRuntimePublisherStub struct {
	err      error
	topics   []string
	messages []*cbmessaging.Message
}

func (p *eventRuntimePublisherStub) Publish(ctx context.Context, topic string, body []byte) error {
	return p.PublishMessage(ctx, topic, cbmessaging.NewMessage("raw", body))
}

func (p *eventRuntimePublisherStub) PublishMessage(_ context.Context, topic string, msg *cbmessaging.Message) error {
	p.topics = append(p.topics, topic)
	p.messages = append(p.messages, msg)
	return p.err
}

func (p *eventRuntimePublisherStub) Close() error {
	return nil
}

type unknownEvent struct{}

func (unknownEvent) EventID() string       { return "unknown-1" }
func (unknownEvent) EventType() string     { return "iam.unknown" }
func (unknownEvent) OccurredAt() time.Time { return time.Now() }
func (unknownEvent) AggregateType() string { return "Unknown" }
func (unknownEvent) AggregateID() string   { return "unknown" }
func (unknownEvent) Payload() any          { return map[string]string{"ok": "true"} }
