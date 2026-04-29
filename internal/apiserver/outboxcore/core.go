package outboxcore

import (
	"time"

	outboxport "github.com/FangcunMount/iam/internal/apiserver/port/outbox"
	"github.com/FangcunMount/iam/internal/pkg/event"
	"github.com/FangcunMount/iam/internal/pkg/eventcatalog"
	sharedcore "github.com/FangcunMount/iam/pkg/outboxcore"
)

const (
	StatusPending    = sharedcore.StatusPending
	StatusPublishing = sharedcore.StatusPublishing
	StatusPublished  = sharedcore.StatusPublished
	StatusFailed     = sharedcore.StatusFailed

	DefaultPublishingStaleFor = sharedcore.DefaultPublishingStaleFor
	DefaultRelayRetryDelay    = sharedcore.DefaultRelayRetryDelay
)

type Record = sharedcore.Record
type StatusObservation = sharedcore.StatusObservation

type BuildRecordsOptions struct {
	Events   []event.DomainEvent
	Resolver eventcatalog.TopicResolver
	Delivery eventcatalog.DeliveryClassResolver
	Now      time.Time
}

func UnfinishedStatuses() []string {
	return sharedcore.UnfinishedStatuses()
}

func BuildRecords(opts BuildRecordsOptions) ([]Record, error) {
	return sharedcore.BuildRecords(sharedcore.BuildRecordsOptions{
		Events:   opts.Events,
		Resolver: opts.Resolver,
		Delivery: opts.Delivery,
		Now:      opts.Now,
	})
}

func BuildStatusSnapshot(store string, now time.Time, observations []StatusObservation) outboxport.StatusSnapshot {
	return sharedcore.BuildStatusSnapshot(store, now, observations)
}
