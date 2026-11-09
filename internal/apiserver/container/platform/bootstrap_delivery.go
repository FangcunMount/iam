package platform

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/bootstrapdelivery"
	"github.com/FangcunMount/reliable-messaging/transaction"
)

var errBootstrapDeliveryPending = errors.New("fresh bootstrap policy notification handoff is pending")

type bootstrapDelivery struct {
	delivery         *bootstrapdelivery.Delivery
	interval, budget time.Duration
	stop             chan struct{}
	done             chan struct{}
	stopOnce         sync.Once
	complete         atomic.Bool
	started          bool // written by Start, read only after the owner's startDone
}

func (b *bootstrapDelivery) check(ctx context.Context) error {
	if b == nil {
		return nil
	}
	check, cancel := context.WithTimeout(ctx, b.budget)
	claimed, err := b.delivery.CheckClaim(check)
	cancel()
	if err != nil {
		return err
	}
	if !claimed {
		b.complete.Store(true)
	}
	return nil
}

func (b *bootstrapDelivery) start() {
	if b == nil || b.ready() {
		return
	}
	b.started = true
	go b.run()
}

func (b *bootstrapDelivery) run() {
	defer close(b.done)
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()
	failed := false
	for {
		select {
		case <-b.stop:
			return
		default:
		}
		// Stop seals future work. An already admitted call owns its real
		// result until it returns or reaches its configured budget.
		check, cancel := context.WithTimeout(context.Background(), b.budget)
		complete, err := b.delivery.Advance(check)
		cancel()
		if err != nil && !failed && !errors.Is(err, ErrTransactionAdmissionClosed) {
			log.Warnw("fresh bootstrap notification handoff failed", "error", err)
		}
		failed = err != nil
		if err == nil && complete {
			b.complete.Store(true)
			return
		}
		select {
		case <-b.stop:
			return
		case <-ticker.C:
		}
	}
}

func (b *bootstrapDelivery) ready() bool { return b == nil || b.complete.Load() }

func (b *bootstrapDelivery) stopAdmission() {
	if b != nil {
		b.stopOnce.Do(func() { close(b.stop) })
	}
}

func (b *bootstrapDelivery) close(ctx context.Context) error {
	b.stopAdmission()
	if b == nil || !b.started {
		return nil
	}
	select {
	case <-b.done:
		return nil
	default:
	}
	select {
	case <-b.done:
		return nil
	case <-ctx.Done():
		return &transaction.ShutdownIncomplete{RuntimePending: true, Cause: ctx.Err()}
	}
}
