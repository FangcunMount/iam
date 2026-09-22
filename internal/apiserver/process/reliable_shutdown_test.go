package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/FangcunMount/component-base/pkg/processruntime"
	"github.com/FangcunMount/component-base/pkg/shutdown"
	messagingInfra "github.com/FangcunMount/iam/v5/internal/apiserver/infra/messaging"
	"github.com/stretchr/testify/require"
)

func TestReliableShutdownFailureExitsNonzero(t *testing.T) {
	if os.Getenv("IAM_RM_SHUTDOWN_CHILD") == "1" {
		gs := shutdown.New()
		configureReliableShutdownFailure(gs, true)
		gs.AddShutdownCallback(shutdown.ShutdownFunc(func(string) error {
			return runShutdownSequence(shutdownSequenceDeps{
				reliableShutdownTimeout: time.Millisecond,
				stopReliable:            func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() },
				closeDatabase:           func() error { os.Exit(2); return nil },
			})
		}))
		gs.StartShutdown(exitProofManager{})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestReliableShutdownFailureExitsNonzero$")
	cmd.Env = append(os.Environ(), "IAM_RM_SHUTDOWN_CHILD=1")
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	require.ErrorAs(t, err, &exit, string(output))
	require.Equal(t, 1, exit.ExitCode(), string(output))
}

type exitProofManager struct{}

func (exitProofManager) GetName() string                  { return "proof" }
func (exitProofManager) Start(shutdown.GSInterface) error { return nil }
func (exitProofManager) ShutdownStart() error             { return nil }
func (exitProofManager) ShutdownFinish() error            { os.Exit(0); return nil }

// Runs the production shutdown sequence against the real host runtime adapter.
// Controlled Run/Drain boundaries are not real NSQ/SQL fault evidence.
func TestReliableShutdownRetainsResourcesUntilRetryDrains(t *testing.T) {
	runner := &shutdownReliableRunner{entered: make(chan struct{}), canceled: make(chan struct{}), release: make(chan struct{})}
	publisher := &shutdownReliableDrain{fail: true}
	runtime, err := messagingInfra.NewReliableRuntime(runner, publisher, time.Second)
	require.NoError(t, err)
	released := false
	t.Cleanup(func() {
		if !released {
			close(runner.release)
		}
		publisher.fail = false
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, runtime.Stop(ctx))
	})
	require.NoError(t, runtime.Start(context.Background()))
	waitReliableShutdownSignal(t, runner.entered)
	var order []string
	var lifecycle processruntime.Lifecycle
	lifecycle.AddShutdownHook("remaining lifecycle", func() error { order = append(order, "hooks"); return nil })
	deps := shutdownSequenceDeps{
		lifecycle: lifecycle, stopReliable: runtime.Stop, reliableShutdownTimeout: 20 * time.Millisecond,
		closeReliableProducer: func() { order = append(order, "producer") },
		closeDatabase:         func() error { order = append(order, "database"); return nil },
	}
	err = runShutdownSequence(deps)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	waitReliableShutdownSignal(t, runner.canceled)
	require.Empty(t, order, "join timeout must retain resources and not run later hooks")
	close(runner.release)
	released = true
	deps.reliableShutdownTimeout = time.Second
	err = runShutdownSequence(deps)
	require.ErrorContains(t, err, "held driver")
	require.Empty(t, order, "publisher drain failure must retain resources")
	publisher.fail = false
	require.NoError(t, runShutdownSequence(deps))
	require.Equal(t, []string{"producer", "hooks", "database"}, order)
}

func waitReliableShutdownSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatal("shutdown boundary did not complete")
	}
}

type shutdownReliableRunner struct{ entered, canceled, release chan struct{} }

func (r *shutdownReliableRunner) Run(ctx context.Context) error {
	close(r.entered)
	<-ctx.Done()
	close(r.canceled)
	<-r.release
	return nil
}

type shutdownReliableDrain struct{ fail bool }

func (d *shutdownReliableDrain) Drain(context.Context) error {
	if d.fail {
		return errors.New("held driver")
	}
	return nil
}
