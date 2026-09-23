//go:build reliable_messaging

package process

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/FangcunMount/component-base/pkg/processruntime"
)

// This is a host lifecycle characterization, not an NSQ or SQL durability test.
// It uses the actual legacy scheduling loop and shutdown sequence. The hook is
// explicitly supplied: cancel-only mirrors current startRuntimeTasks; cancel +
// join is the proposed integration contract, not production wiring.
func TestReliableMessagingShutdownJoinBoundary(t *testing.T) {
	for _, join := range []bool{false, true} {
		name := "legacy_cancel_only"
		if join {
			name = "candidate_cancel_and_join"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			dispatch := &heldMessagingDispatch{entered: make(chan struct{}), canceled: make(chan struct{}), release: make(chan struct{})}
			relayDone := make(chan struct{})
			shutdownDone := make(chan struct{})
			dbClosed := make(chan struct{})
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(dispatch.release) }) }
			go func() { var server *apiServer; server.runOutboxRelay(ctx, dispatch); close(relayDone) }()
			t.Cleanup(func() { cancel(); release(); awaitMessagingSignal(t, relayDone, "relay cleanup") })
			awaitMessagingSignal(t, dispatch.entered, "admitted dispatch")
			var lifecycle processruntime.Lifecycle
			lifecycle.AddShutdownHook("stop outbox relay", func() error {
				cancel()
				if join {
					<-relayDone
				}
				return nil
			})
			go func() {
				if err := runShutdownSequence(shutdownSequenceDeps{lifecycle: lifecycle, closeDatabase: func() error { close(dbClosed); return nil }}); err != nil {
					t.Errorf("shutdown: %v", err)
				}
				close(shutdownDone)
			}()
			t.Cleanup(func() { release(); awaitMessagingSignal(t, shutdownDone, "shutdown cleanup") })
			awaitMessagingSignal(t, dispatch.canceled, "dispatch observed cancellation")
			if join {
				select {
				case <-dbClosed:
					t.Fatal("database closed before admitted dispatch drained")
				default:
				}
			} else {
				awaitMessagingSignal(t, dbClosed, "legacy database close")
				select {
				case <-relayDone:
					t.Fatal("fixture did not hold dispatch through database close")
				default:
				}
				t.Log("OBSERVED LIMIT: legacy cancel-only shutdown closes database while dispatch remains active")
			}
			release()
			awaitMessagingSignal(t, relayDone, "drained relay")
			awaitMessagingSignal(t, shutdownDone, "completed shutdown")
			awaitMessagingSignal(t, dbClosed, "database close after shutdown")
		})
	}
}

type heldMessagingDispatch struct{ entered, canceled, release chan struct{} }

func (d *heldMessagingDispatch) DispatchDue(ctx context.Context) error {
	close(d.entered)
	<-ctx.Done()
	close(d.canceled)
	// Model a cooperative admitted operation that still has completion/writeback
	// work after admission cancellation; the test controls that completion.
	<-d.release
	return nil
}
func awaitMessagingSignal(t *testing.T, signal <-chan struct{}, stage string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for %s", stage)
	}
}
