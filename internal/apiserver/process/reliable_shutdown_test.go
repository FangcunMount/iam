package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/FangcunMount/component-base/pkg/shutdown"
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

func TestPolicySubscriberDrainGatesMySQLClose(t *testing.T) {
	var order []string
	fail := true
	deps := shutdownSequenceDeps{
		reliableShutdownTimeout: time.Second,
		stopPolicySync: func(context.Context) error {
			order = append(order, "policy subscriber")
			if fail {
				return errors.New("failed audit still in flight")
			}
			return nil
		},
		stopReliable:  func(context.Context) error { order = append(order, "relay"); return nil },
		closeDatabase: func() error { order = append(order, "mysql"); return nil },
	}
	require.ErrorContains(t, runShutdownSequence(deps), "failed audit still in flight")
	require.Equal(t, []string{"policy subscriber"}, order)
	fail = false
	require.NoError(t, runShutdownSequence(deps))
	require.Equal(t, []string{"policy subscriber", "policy subscriber", "relay", "mysql"}, order)
}
