package messaging

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FangcunMount/reliable-messaging/message"
	"github.com/FangcunMount/reliable-messaging/outbox"
	"github.com/FangcunMount/reliable-messaging/relay"
	"github.com/FangcunMount/reliable-messaging/transport"
	"github.com/stretchr/testify/require"
)

// The actual SDK Relay is used; SQL and NSQ operations are controlled doubles.
// This proves supervision and ordering, not database/broker crash recovery.
func TestReliableRuntimeRestartsSDKAndDrainsAdmittedWork(t *testing.T) {
	m, err := message.New(message.Input{Producer: "iam", ID: "intent-1", Destination: "policy", EventType: "changed", SchemaVersion: "v2", Scope: "global", ContentType: "application/json", OccurredAt: "2026-09-22T00:00:00Z", Payload: []byte(`{"version":1}`)})
	require.NoError(t, err)
	store := &runtimeStore{intent: m, confirmed: make(chan struct{})}
	publisher := &runtimePublisher{entered: make(chan struct{}), release: make(chan struct{}), driverDone: make(chan struct{}), drainEntered: make(chan struct{}, 4)}
	sdk, err := relay.New(store, publisher, relay.Config{
		Concurrency: 1, PollInterval: time.Millisecond, Lease: 10 * time.Second,
		PublishTimeout: 5 * time.Second, WriteTimeout: time.Second,
		Retry: func(outbox.Claim, transport.Outcome) relay.RetryDecision {
			return relay.RetryDecision{Delay: time.Second}
		},
		Observe: func(relay.Event) {},
	})
	require.NoError(t, err)
	runtime, err := NewReliableRuntime(sdk, publisher, time.Millisecond)
	require.NoError(t, err)
	parent, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	// Ensure test failures cannot leak an admitted operation.
	t.Cleanup(func() {
		select {
		case <-publisher.release:
		default:
			close(publisher.release)
		}
		select {
		case <-publisher.driverDone:
		default:
			close(publisher.driverDone)
		}
		ctx, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		require.NoError(t, runtime.Stop(ctx))
	})
	require.NoError(t, runtime.Start(parent))
	require.Error(t, runtime.Start(parent))
	awaitRuntimeSignal(t, publisher.entered)
	require.EqualValues(t, 2, store.scans.Load(), "failed scan must restart and admit original intent")

	// Stop cancels admission, but must neither return success nor drain the
	// publisher while the admitted SDK publish/write remains outstanding.
	expired, expire := context.WithCancel(context.Background())
	expire()
	require.ErrorIs(t, runtime.Stop(expired), context.Canceled)
	require.Empty(t, publisher.drainEntered)
	require.Error(t, runtime.Start(parent))
	close(publisher.release)
	awaitRuntimeSignal(t, store.confirmed)
	require.False(t, store.writeCanceled.Load(), "SDK writeback must survive admission cancellation")
	awaitRuntimeSignal(t, runtime.done)

	// A completed Relay does not imply all underlying driver calls drained.
	drainCtx, cancelDrain := context.WithCancel(context.Background())
	drainResult := make(chan error, 1)
	go func() { drainResult <- runtime.Stop(drainCtx) }()
	awaitRuntimeSignal(t, publisher.drainEntered)
	cancelDrain()
	select {
	case err := <-drainResult:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("drain timeout did not return")
	}
	close(publisher.driverDone)
	ctx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	require.NoError(t, runtime.Stop(ctx))
	require.NoError(t, runtime.Stop(ctx))
	require.EqualValues(t, 2, store.scans.Load(), "no new admission after cancellation")
}

func TestReliableRuntimeStopInterruptsBackoffAndPreventsRestart(t *testing.T) {
	for _, runErr := range []error{errors.New("scan failed"), nil} {
		runner := &returningRuntimeRunner{result: runErr, entered: make(chan struct{})}
		publisher := &immediateRuntimeDrain{}
		runtime, err := NewReliableRuntime(runner, publisher, time.Minute)
		require.NoError(t, err)
		require.NoError(t, runtime.Start(context.Background()))
		awaitRuntimeSignal(t, runner.entered)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		require.NoError(t, runtime.Stop(ctx))
		cancel()
		require.EqualValues(t, 1, runner.calls.Load())
		require.EqualValues(t, 1, publisher.calls.Load())
	}
}

func TestReliableRuntimeStopBeforeStartAndCanceledStart(t *testing.T) {
	runner := &returningRuntimeRunner{entered: make(chan struct{})}
	publisher := &immediateRuntimeDrain{}
	runtime, err := NewReliableRuntime(runner, publisher, time.Second)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, runtime.Start(ctx), context.Canceled)
	require.NoError(t, runtime.Stop(context.Background()))
	require.Error(t, runtime.Start(context.Background()))
	require.Zero(t, runner.calls.Load())
}

type runtimeStore struct {
	intent        message.Message
	scans         atomic.Int32
	confirmed     chan struct{}
	writeCanceled atomic.Bool
}

func (s *runtimeStore) ClaimDue(ctx context.Context, _ int, _ time.Duration) ([]outbox.Claim, error) {
	switch s.scans.Add(1) {
	case 1:
		return nil, errors.New("temporary scan failure")
	case 2:
		return []outbox.Claim{{Message: s.intent}}, nil
	default:
		<-ctx.Done()
		return nil, ctx.Err()
	}
}
func (s *runtimeStore) Confirm(ctx context.Context, _ outbox.Claim) error {
	s.writeCanceled.Store(ctx.Err() != nil)
	close(s.confirmed)
	return nil
}
func (*runtimeStore) Retry(context.Context, outbox.Claim, time.Duration, string) error {
	return errors.New("unexpected retry")
}
func (*runtimeStore) Quarantine(context.Context, outbox.Claim, string) error {
	return errors.New("unexpected quarantine")
}

type runtimePublisher struct{ entered, release, driverDone, drainEntered chan struct{} }

func (p *runtimePublisher) Publish(ctx context.Context, _ message.Message) transport.Result {
	close(p.entered)
	select {
	case <-p.release:
		return transport.Result{Outcome: transport.Confirmed}
	case <-ctx.Done():
		return transport.Result{Outcome: transport.Unknown}
	}
}
func (p *runtimePublisher) Drain(ctx context.Context) error {
	p.drainEntered <- struct{}{}
	select {
	case <-p.driverDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type returningRuntimeRunner struct {
	result  error
	entered chan struct{}
	calls   atomic.Int32
}

func (r *returningRuntimeRunner) Run(context.Context) error {
	if r.calls.Add(1) == 1 {
		close(r.entered)
	}
	return r.result
}

type immediateRuntimeDrain struct{ calls atomic.Int32 }

func (d *immediateRuntimeDrain) Drain(context.Context) error { d.calls.Add(1); return nil }

func awaitRuntimeSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for runtime boundary")
	}
}
