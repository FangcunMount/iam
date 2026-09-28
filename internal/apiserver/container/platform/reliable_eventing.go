package platform

import (
	"context"
	"errors"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	messagingInfra "github.com/FangcunMount/iam/v5/internal/apiserver/infra/messaging"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/pkg/outboxcore"
	"github.com/FangcunMount/reliable-messaging/outbox"
	"github.com/FangcunMount/reliable-messaging/relay"
	sdkmysql "github.com/FangcunMount/reliable-messaging/storage/mysql"
	"github.com/FangcunMount/reliable-messaging/transport"
	nsqtransport "github.com/FangcunMount/reliable-messaging/transport/nsq"
	"github.com/nsqio/go-nsq"
)

// managedPolicyTransport lets the IAM Relay use its existing drain contract
// while the SDK owns and closes the NSQ producer after admitted sends finish.
type managedPolicyTransport struct{ *nsqtransport.ManagedPublisher }

func (p managedPolicyTransport) Drain(ctx context.Context) error {
	return p.ManagedPublisher.Close(ctx)
}

func initReliableEventing(deps EventingDeps, result *Eventing) error {
	opts := deps.ReliableMessaging
	if err := opts.Validate(); err != nil {
		return err
	}
	if !deps.NSQEnabled || deps.NSQAddress == "" {
		return errors.New("reliable messaging requires configured NSQ")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := eventoutbox.CheckReliableSchema(ctx, deps.DB); err != nil {
		return err
	}
	stager, err := eventoutbox.NewStandardStager(result.Catalog)
	if err != nil {
		return err
	}
	topic, _ := result.Catalog.GetTopicForEvent(eventing.AuthzVersionChanged)
	if err := eventoutbox.CheckLegacyDrained(ctx, deps.DB); err != nil {
		return err
	}
	pool, err := deps.DB.DB()
	if err != nil {
		return err
	}
	store, err := sdkmysql.New(pool)
	if err != nil {
		return err
	}
	// The SDK owns this dedicated NSQ connection. IAM still owns its policy
	// transaction, original wire envelope and Relay recovery decisions.
	cfg := nsq.NewConfig()
	cfg.DialTimeout = opts.PublishTimeout
	cfg.ReadTimeout = opts.PublishTimeout
	cfg.WriteTimeout = opts.PublishTimeout
	cfg.HeartbeatInterval = min(opts.PublishTimeout/2, 30*time.Second)
	managed, err := nsqtransport.NewManagedPublisher(nsqtransport.ManagedPublisherConfig{
		Address: deps.NSQAddress, Driver: cfg, Routes: map[string]string{topic: topic}, MaxInFlight: opts.Concurrency,
	})
	if err != nil {
		return err
	}
	success := false
	defer func() {
		if !success {
			closeCtx, cancel := context.WithTimeout(context.Background(), opts.PublishTimeout)
			if managed.Close(closeCtx) != nil {
				managed.Interrupt()
				cancel()
				closeCtx, cancel = context.WithTimeout(context.Background(), opts.PublishTimeout)
				_ = managed.Close(closeCtx)
			}
			cancel()
		}
	}()
	directPublisher, err := messagingInfra.NewDirectEventPublisher(result.Catalog, managed, eventing.SourceAPIServer)
	if err != nil {
		return err
	}
	publisher, err := messagingInfra.NewPolicyWirePublisher(managedPolicyTransport{managed})
	if err != nil {
		return err
	}
	poll := deps.OutboxInterval
	if poll <= 0 {
		poll = 2 * time.Second
	}
	retry := deps.OutboxRetry
	if retry <= 0 {
		retry = outboxcore.DefaultRelayRetryDelay
	}
	runner, err := relay.New(store, publisher, relay.Config{
		Concurrency: opts.Concurrency, PollInterval: poll, Lease: opts.Lease,
		PublishTimeout: opts.PublishTimeout, WriteTimeout: opts.WriteTimeout,
		Retry: func(_ outbox.Claim, outcome transport.Outcome) relay.RetryDecision {
			// Policy notification duplicates retain the original identity and are
			// version-idempotent. Unknown here never authorizes an AI model retry.
			return relay.RetryDecision{Delay: retry, Quarantine: outcome == transport.Rejected}
		},
		Observe: func(e relay.Event) {
			if e.Err != nil {
				log.Warnw("reliable policy delivery", "kind", e.Kind, "outcome", e.Outcome)
			}
		},
	})
	if err != nil {
		return err
	}
	runtime, err := messagingInfra.NewReliableRuntime(runner, publisher, opts.RestartDelay)
	if err != nil {
		return err
	}
	result.Stager = stager
	result.Publisher = directPublisher
	result.Outbox = eventoutbox.NewStandardStatusReader(deps.DB)
	result.ReliableRuntime = runtime
	success = true
	return nil
}
