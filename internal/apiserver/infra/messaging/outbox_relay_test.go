package messaging

import (
	"context"
	"errors"
	"testing"
	"time"

	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
	outboxport "github.com/FangcunMount/iam/pkg/outbox"
	"github.com/stretchr/testify/require"
)

type relayStoreStub struct {
	pending     []outboxport.PendingEvent
	claimCalled int
	published   []string
	failed      []string
}

func (s *relayStoreStub) ClaimDueEvents(context.Context, int, time.Time) ([]outboxport.PendingEvent, error) {
	s.claimCalled++
	return s.pending, nil
}

func (s *relayStoreStub) MarkEventPublished(_ context.Context, eventID string, _ time.Time) error {
	s.published = append(s.published, eventID)
	return nil
}

func (s *relayStoreStub) MarkEventFailed(_ context.Context, eventID, _ string, _ time.Time) error {
	s.failed = append(s.failed, eventID)
	return nil
}

type relayPublisherStub struct {
	err       error
	topics    []string
	messages  []*cbmessaging.Message
	closeCall int
}

func (p *relayPublisherStub) Publish(ctx context.Context, topic string, body []byte) error {
	return p.PublishMessage(ctx, topic, cbmessaging.NewMessage("raw", body))
}

func (p *relayPublisherStub) PublishMessage(_ context.Context, topic string, msg *cbmessaging.Message) error {
	p.topics = append(p.topics, topic)
	p.messages = append(p.messages, msg)
	return p.err
}

func (p *relayPublisherStub) Close() error {
	p.closeCall++
	return nil
}

type relayBusStub struct {
	publisher cbmessaging.Publisher
}

func (b relayBusStub) Publisher() cbmessaging.Publisher {
	return b.publisher
}

func (b relayBusStub) Subscriber() cbmessaging.Subscriber {
	return nil
}

func (b relayBusStub) Router() *cbmessaging.Router {
	return nil
}

func (b relayBusStub) Health() error {
	return nil
}

func (b relayBusStub) Close() error {
	return nil
}

func TestOutboxRelayDegradesWithoutEventBus(t *testing.T) {
	store := &relayStoreStub{pending: []outboxport.PendingEvent{{EventID: "evt-1"}}}
	relay := NewOutboxRelay("test", store, nil)

	require.NoError(t, relay.DispatchDue(context.Background()))

	require.Zero(t, store.claimCalled)
	require.Empty(t, store.published)
	require.Empty(t, store.failed)
}

func TestOutboxRelayPublishesAndMarksPublished(t *testing.T) {
	store := &relayStoreStub{pending: []outboxport.PendingEvent{{
		EventID:       "evt-1",
		EventType:     "iam.authz.version_changed",
		AggregateType: "PolicyVersion",
		AggregateID:   "tenant-a:1",
		TopicName:     "iam.authz.version",
		Payload:       []byte(`{"tenant_id":"tenant-a","version":1}`),
	}}}
	publisher := &relayPublisherStub{}
	relay := NewOutboxRelay("test", store, relayBusStub{publisher: publisher})

	require.NoError(t, relay.DispatchDue(context.Background()))

	require.Equal(t, []string{"iam.authz.version"}, publisher.topics)
	require.Len(t, publisher.messages, 1)
	require.Equal(t, "evt-1", publisher.messages[0].UUID)
	require.Equal(t, "iam.authz.version_changed", publisher.messages[0].Metadata["event_type"])
	require.Equal(t, []string{"evt-1"}, store.published)
	require.Empty(t, store.failed)
}

func TestOutboxRelayMarksFailedWhenPublishFails(t *testing.T) {
	store := &relayStoreStub{pending: []outboxport.PendingEvent{{
		EventID:   "evt-1",
		EventType: "iam.authz.version_changed",
		TopicName: "iam.authz.version",
		Payload:   []byte(`{}`),
	}}}
	publisher := &relayPublisherStub{err: errors.New("mq down")}
	relay := NewOutboxRelay("test", store, relayBusStub{publisher: publisher})

	require.NoError(t, relay.DispatchDue(context.Background()))

	require.Empty(t, store.published)
	require.Equal(t, []string{"evt-1"}, store.failed)
}
