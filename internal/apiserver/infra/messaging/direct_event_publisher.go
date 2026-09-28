package messaging

import (
	"context"
	"errors"
	"fmt"

	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/FangcunMount/iam/v5/pkg/eventcodec"
	"github.com/FangcunMount/reliable-messaging/transport"
	"github.com/FangcunMount/reliable-messaging/wire/legacy"
)

var (
	ErrDirectPublishUnknown  = errors.New("direct event publish outcome unknown")
	ErrDirectPublishRejected = errors.New("direct event publish rejected")
)

type directEventTransport interface {
	PublishRaw(context.Context, string, []byte) transport.Result
}

// DirectEventPublisher sends IAM's best-effort catalog events through the
// SDK-owned producer. Durable events must be staged in the original transaction.
type DirectEventPublisher struct {
	catalog *eventcatalog.Catalog
	next    directEventTransport
	source  string
}

func NewDirectEventPublisher(catalog *eventcatalog.Catalog, next directEventTransport, source string) (*DirectEventPublisher, error) {
	if catalog == nil || next == nil {
		return nil, errors.New("direct event catalog and transport required")
	}
	if source == "" {
		source = event.SourceDefault
	}
	return &DirectEventPublisher{catalog: catalog, next: next, source: source}, nil
}

func (p *DirectEventPublisher) Publish(ctx context.Context, evt event.DomainEvent) error {
	if evt == nil {
		return nil
	}
	topic, ok := p.catalog.GetTopicForEvent(evt.EventType())
	if !ok {
		return fmt.Errorf("event type %q not found in catalog", evt.EventType())
	}
	if delivery, ok := p.catalog.GetDeliveryClass(evt.EventType()); ok && delivery == eventcatalog.DeliveryClassDurableOutbox {
		return fmt.Errorf("event type %q is durable_outbox and must be staged to outbox", evt.EventType())
	}
	payload, err := eventcodec.EncodePayload(evt)
	if err != nil {
		return err
	}
	wire, err := legacy.Encode(legacy.Envelope{
		UUID: evt.EventID(), Metadata: eventcodec.MetadataFromEvent(evt, p.source), Payload: payload,
	}, legacy.Revision1)
	if err != nil {
		return err
	}
	switch p.next.PublishRaw(ctx, topic, wire).Outcome {
	case transport.Confirmed:
		return nil
	case transport.Rejected:
		return fmt.Errorf("%w: event type %s", ErrDirectPublishRejected, evt.EventType())
	default:
		// The broker may already have the message. No automatic retry or new ID.
		return fmt.Errorf("%w: event type %s", ErrDirectPublishUnknown, evt.EventType())
	}
}

func (p *DirectEventPublisher) PublishAll(ctx context.Context, events []event.DomainEvent) error {
	for _, evt := range events {
		if err := p.Publish(ctx, evt); err != nil {
			return err
		}
	}
	return nil
}
