package eventruntime

import (
	"context"
	"fmt"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/v2/pkg/event"
	"github.com/FangcunMount/iam/v2/pkg/eventcatalog"
	"github.com/FangcunMount/iam/v2/pkg/eventmessaging"
)

type PublishMode string

const (
	PublishModeMQ      PublishMode = "mq"
	PublishModeLogging PublishMode = "logging"
	PublishModeNop     PublishMode = "nop"
)

type RoutingPublisher struct {
	topicResolver eventcatalog.TopicResolver
	delivery      eventcatalog.DeliveryClassResolver
	mqPublisher   messaging.Publisher
	source        string
	mode          PublishMode
}

type RoutingPublisherOptions struct {
	Catalog       *eventcatalog.Catalog
	TopicResolver eventcatalog.TopicResolver
	Delivery      eventcatalog.DeliveryClassResolver
	MQPublisher   messaging.Publisher
	Source        string
	Mode          PublishMode
}

func NewRoutingPublisher(opts RoutingPublisherOptions) *RoutingPublisher {
	resolver := opts.TopicResolver
	if resolver == nil && opts.Catalog != nil {
		resolver = opts.Catalog
	}
	delivery := opts.Delivery
	if delivery == nil && opts.Catalog != nil {
		delivery = opts.Catalog
	}
	if resolver == nil {
		resolver = eventcatalog.NewCatalog(nil)
	}
	source := opts.Source
	if source == "" {
		source = event.SourceDefault
	}
	mode := opts.Mode
	if mode == "" {
		mode = PublishModeLogging
	}
	return &RoutingPublisher{
		topicResolver: resolver,
		delivery:      delivery,
		mqPublisher:   opts.MQPublisher,
		source:        source,
		mode:          mode,
	}
}

func NewPublisherForBus(catalog *eventcatalog.Catalog, bus messaging.EventBus, source string) event.Publisher {
	var mq messaging.Publisher
	mode := PublishModeLogging
	if bus != nil {
		mq = bus.Publisher()
		mode = PublishModeMQ
	}
	return NewRoutingPublisher(RoutingPublisherOptions{
		Catalog:     catalog,
		MQPublisher: mq,
		Source:      source,
		Mode:        mode,
	})
}

func (p *RoutingPublisher) Publish(ctx context.Context, evt event.DomainEvent) error {
	if evt == nil {
		return nil
	}
	topicName, ok := p.topicResolver.GetTopicForEvent(evt.EventType())
	if !ok {
		return fmt.Errorf("event type %q not found in catalog", evt.EventType())
	}
	if p.delivery != nil {
		delivery, ok := p.delivery.GetDeliveryClass(evt.EventType())
		if ok && delivery == eventcatalog.DeliveryClassDurableOutbox {
			return fmt.Errorf("event type %q is durable_outbox and must be staged to outbox", evt.EventType())
		}
	}
	switch p.mode {
	case PublishModeMQ:
		if p.mqPublisher == nil {
			log.Warnf("event publisher has no MQ publisher; event logged only: %s", evt.EventType())
			return nil
		}
		msg, err := eventmessaging.BuildMessage(evt, p.source)
		if err != nil {
			return err
		}
		return p.mqPublisher.PublishMessage(ctx, topicName, msg)
	case PublishModeNop:
		return nil
	default:
		log.Infof("[DomainEvent] type=%s id=%s topic=%s aggregate=%s/%s",
			evt.EventType(), evt.EventID(), topicName, evt.AggregateType(), evt.AggregateID())
		return nil
	}
}

func (p *RoutingPublisher) PublishAll(ctx context.Context, events []event.DomainEvent) error {
	for _, evt := range events {
		if err := p.Publish(ctx, evt); err != nil {
			return err
		}
	}
	return nil
}
