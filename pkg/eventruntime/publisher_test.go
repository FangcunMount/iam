package eventruntime

import (
	"context"
	"errors"
	"testing"

	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/v2/pkg/event"
	"github.com/FangcunMount/iam/v2/pkg/eventcatalog"
	"github.com/stretchr/testify/require"
)

func TestRoutingPublisherRejectsDurableOutboxEvents(t *testing.T) {
	catalog := testEventRuntimeCatalog(t)
	publisher := NewRoutingPublisher(RoutingPublisherOptions{
		Catalog:     catalog,
		MQPublisher: &eventRuntimePublisherStub{},
		Mode:        PublishModeMQ,
	})

	err := publisher.Publish(context.Background(), event.New("test.durable", "Version", "v-1", map[string]int{"version": 1}))

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

	err := publisher.Publish(context.Background(), event.New("test.best_effort", "Notification", "n-1", map[string]string{"message": "hello"}))

	require.NoError(t, err)
	require.Equal(t, []string{"test.notify"}, mq.topics)
	require.Len(t, mq.messages, 1)
	require.Equal(t, "test.best_effort", mq.messages[0].Metadata["event_type"])
}

func TestRoutingPublisherPropagatesPublishError(t *testing.T) {
	catalog := testEventRuntimeCatalog(t)
	publisher := NewRoutingPublisher(RoutingPublisherOptions{
		Catalog:     catalog,
		MQPublisher: &eventRuntimePublisherStub{err: errors.New("mq down")},
		Mode:        PublishModeMQ,
	})

	err := publisher.Publish(context.Background(), event.New("test.best_effort", "Notification", "n-1", map[string]string{"message": "hello"}))

	require.ErrorContains(t, err, "mq down")
}

func testEventRuntimeCatalog(t *testing.T) *eventcatalog.Catalog {
	t.Helper()
	cfg, err := eventcatalog.Parse([]byte(`
version: "1"
topics:
  durable:
    name: test.durable
  notify:
    name: test.notify
events:
  test.durable:
    topic: durable
    delivery: durable_outbox
    aggregate: Version
    domain: test
    handler: test-sync
  test.best_effort:
    topic: notify
    delivery: best_effort
    aggregate: Notification
    domain: test
    handler: test-notifier
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
