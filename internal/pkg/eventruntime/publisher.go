package eventruntime

import (
	"context"
	"fmt"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/internal/pkg/event"
	"github.com/FangcunMount/iam/internal/pkg/eventcatalog"
	"github.com/FangcunMount/iam/internal/pkg/eventcodec"
)

type PublishMode string

const (
	PublishModeMQ      PublishMode = "mq"
	PublishModeLogging PublishMode = "logging"
	PublishModeNop     PublishMode = "nop"
)

type RoutingPublisher struct {
	topicResolver eventcatalog.TopicResolver
	mqPublisher   messaging.Publisher
	source        string
	mode          PublishMode
}

type RoutingPublisherOptions struct {
	Catalog       *eventcatalog.Catalog
	TopicResolver eventcatalog.TopicResolver
	MQPublisher   messaging.Publisher
	Source        string
	Mode          PublishMode
}

func NewRoutingPublisher(opts RoutingPublisherOptions) *RoutingPublisher {
	resolver := opts.TopicResolver
	if resolver == nil && opts.Catalog != nil {
		resolver = opts.Catalog
	}
	if resolver == nil {
		resolver = eventcatalog.NewCatalog(nil)
	}
	source := opts.Source
	if source == "" {
		source = event.SourceAPIServer
	}
	mode := opts.Mode
	if mode == "" {
		mode = PublishModeLogging
	}
	return &RoutingPublisher{
		topicResolver: resolver,
		mqPublisher:   opts.MQPublisher,
		source:        source,
		mode:          mode,
	}
}

func NewPublisherForBus(catalog *eventcatalog.Catalog, bus messaging.EventBus) event.Publisher {
	var mq messaging.Publisher
	mode := PublishModeLogging
	if bus != nil {
		mq = bus.Publisher()
		mode = PublishModeMQ
	}
	return NewRoutingPublisher(RoutingPublisherOptions{
		Catalog:     catalog,
		MQPublisher: mq,
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
	switch p.mode {
	case PublishModeMQ:
		if p.mqPublisher == nil {
			log.Warnf("event publisher has no MQ publisher; event logged only: %s", evt.EventType())
			return nil
		}
		msg, err := eventcodec.BuildMessage(evt, p.source)
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
