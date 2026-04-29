package messaging

import (
	"context"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/internal/apiserver/outboxcore"
	outboxport "github.com/FangcunMount/iam/internal/apiserver/port/outbox"
)

const defaultOutboxRelayBatchSize = 50

type OutboxRelay interface {
	DispatchDue(ctx context.Context) error
}

type OutboxRelayOptions struct {
	BatchSize  int
	RetryDelay time.Duration
}

type outboxRelay struct {
	name       string
	store      outboxport.Store
	publisher  cbmessaging.Publisher
	batchSize  int
	retryDelay time.Duration
}

func NewOutboxRelay(name string, store outboxport.Store, bus cbmessaging.EventBus, opts ...OutboxRelayOptions) OutboxRelay {
	var publisher cbmessaging.Publisher
	if bus != nil {
		publisher = bus.Publisher()
	}
	batchSize := defaultOutboxRelayBatchSize
	retryDelay := outboxcore.DefaultRelayRetryDelay
	if len(opts) > 0 {
		if opts[0].BatchSize > 0 {
			batchSize = opts[0].BatchSize
		}
		if opts[0].RetryDelay > 0 {
			retryDelay = opts[0].RetryDelay
		}
	}
	return &outboxRelay{
		name:       name,
		store:      store,
		publisher:  publisher,
		batchSize:  batchSize,
		retryDelay: retryDelay,
	}
}

func (r *outboxRelay) DispatchDue(ctx context.Context) error {
	if r == nil || r.store == nil {
		return nil
	}
	if r.publisher == nil {
		log.Warnw("outbox relay degraded: event bus unavailable", "relay", r.name)
		return nil
	}
	pendingEvents, err := r.store.ClaimDueEvents(ctx, r.batchSize, time.Now())
	if err != nil {
		return err
	}
	for _, pending := range pendingEvents {
		msg := cbmessaging.NewMessage(pending.EventID, pending.Payload)
		msg.Metadata["event_type"] = pending.EventType
		msg.Metadata["aggregate_type"] = pending.AggregateType
		msg.Metadata["aggregate_id"] = pending.AggregateID
		msg.Metadata["source"] = "iam-outbox-relay"
		if err := r.publisher.PublishMessage(ctx, pending.TopicName, msg); err != nil {
			log.Warnw("outbox publish failed",
				"relay", r.name,
				"event_id", pending.EventID,
				"event_type", pending.EventType,
				"error", err.Error(),
			)
			if markErr := r.store.MarkEventFailed(ctx, pending.EventID, err.Error(), time.Now().Add(r.retryDelay)); markErr != nil {
				log.Errorw("outbox mark failed failed",
					"relay", r.name,
					"event_id", pending.EventID,
					"error", markErr.Error(),
				)
			}
			continue
		}
		if err := r.store.MarkEventPublished(ctx, pending.EventID, time.Now()); err != nil {
			log.Errorw("outbox mark published failed",
				"relay", r.name,
				"event_id", pending.EventID,
				"error", err.Error(),
			)
		}
	}
	return nil
}
