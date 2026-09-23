package messaging

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
)

// ReliableRunner is the SDK's long-running entry point, not DispatchDue.
type ReliableRunner interface {
	Run(context.Context) error
}

// ReliablePublisherDrain waits for real driver calls after Relay has returned.
type ReliablePublisherDrain interface {
	Drain(context.Context) error
}

// ReliableRuntime supervises one host-owned Relay and drains its publisher.
// It owns neither the database nor the producer. Production wiring must only
// close those resources after Stop succeeds. A timeout is not a successful drain;
// the host may interrupt its producer and retry Stop with a new deadline.
type ReliableRuntime struct {
	runner    ReliableRunner
	publisher ReliablePublisherDrain
	restart   time.Duration
	mu        sync.Mutex
	started   bool
	stopping  bool
	cancel    context.CancelFunc
	done      chan struct{}
}

// NewReliableRuntime performs no I/O and starts no goroutines. The caller must
// supply a dedicated Relay and publisher, and exclude every legacy writer.
func NewReliableRuntime(runner ReliableRunner, publisher ReliablePublisherDrain, restart time.Duration) (*ReliableRuntime, error) {
	if runner == nil || publisher == nil || restart <= 0 || restart > time.Minute {
		return nil, errors.New("reliable runner, publisher and restart interval in (0, 1m] required")
	}
	return &ReliableRuntime{runner: runner, publisher: publisher, restart: restart, done: make(chan struct{})}, nil
}

// Start is explicit and single-use, including after Stop. Restarts following
// scan failures occur serially inside supervise; they never overlap SDK Run.
func (r *ReliableRuntime) Start(parent context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started || r.stopping {
		return errors.New("reliable runtime already started or stopped")
	}
	if err := parent.Err(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel
	r.started = true
	go r.supervise(ctx)
	return nil
}

func (r *ReliableRuntime) supervise(ctx context.Context) {
	defer close(r.done)
	for ctx.Err() == nil {
		err := r.runner.Run(ctx)
		if ctx.Err() != nil {
			return
		}
		// Do not log arbitrary driver error text or payloads. SDK observer events
		// provide bounded claim/write categories for separate host diagnostics.
		reason := "runner_error"
		if err == nil {
			reason = "unexpected_return"
		}
		log.Warnw("reliable relay stopped; restart scheduled", "reason", reason, "restart_delay", r.restart)
		timer := time.NewTimer(r.restart)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// Stop first cancels admission, joins the Relay (including admitted writebacks),
// then drains publisher driver calls. Concurrent calls are safe provided the
// borrowed publisher supports concurrent Drain, as the SDK NSQ adapter does.
// Stop before Start permanently closes admission and still drains the publisher.
func (r *ReliableRuntime) Stop(ctx context.Context) error {
	r.mu.Lock()
	if !r.stopping {
		r.stopping = true
		if r.started {
			r.cancel()
		} else {
			close(r.done)
		}
	}
	r.mu.Unlock()
	select {
	case <-r.done:
	case <-ctx.Done():
		return fmt.Errorf("join reliable relay: %w", ctx.Err())
	}
	if err := r.publisher.Drain(ctx); err != nil {
		return fmt.Errorf("drain reliable publisher: %w", err)
	}
	return nil
}
