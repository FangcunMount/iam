package messaging

import (
	"context"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
	outboxport "github.com/FangcunMount/iam/v2/pkg/outbox"
	"github.com/FangcunMount/iam/v2/pkg/outboxcore"
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
	started := time.Now()
	if r.publisher == nil {
		log.WarnContext(ctx, "outbox relay degraded: event bus unavailable",
			log.String("relay", r.name),
			log.String("result", "degraded"),
			log.Int64("duration_ms", time.Since(started).Milliseconds()),
		)
		return nil
	}
	pendingEvents, err := r.store.ClaimDueEvents(ctx, r.batchSize, time.Now())
	if err != nil {
		log.ErrorContext(ctx, "outbox relay claim failed",
			log.String("relay", r.name),
			log.String("result", "failed"),
			log.Int("batch_size", r.batchSize),
			log.Int64("duration_ms", time.Since(started).Milliseconds()),
			log.Err(err),
		)
		return err
	}
	published := 0
	publishFailed := 0
	markFailed := 0
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
				markFailed++
				log.Errorw("outbox mark failed failed",
					"relay", r.name,
					"event_id", pending.EventID,
					"error", markErr.Error(),
				)
			}
			publishFailed++
			continue
		}
		if err := r.store.MarkEventPublished(ctx, pending.EventID, time.Now()); err != nil {
			markFailed++
			log.Errorw("outbox mark published failed",
				"relay", r.name,
				"event_id", pending.EventID,
				"error", err.Error(),
			)
		}
		published++
	}
	if len(pendingEvents) > 0 {
		result := "success"
		if publishFailed > 0 || markFailed > 0 {
			result = "partial_failure"
		}
		log.InfoContext(ctx, "outbox relay dispatch completed",
			log.String("relay", r.name),
			log.String("result", result),
			log.Int("claimed", len(pendingEvents)),
			log.Int("published", published),
			log.Int("publish_failed", publishFailed),
			log.Int("mark_failed", markFailed),
			log.Int64("duration_ms", time.Since(started).Milliseconds()),
		)
	}
	return nil
}
