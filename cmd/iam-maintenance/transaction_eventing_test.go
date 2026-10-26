package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/FangcunMount/iam/v5/internal/apiserver/options"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/reliable-messaging/transaction"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

type maintenanceOwnerStub struct {
	startErr, stopErr error
	starts, stops     int
}

func (s *maintenanceOwnerStub) Start(context.Context) error { s.starts++; return s.startErr }
func (s *maintenanceOwnerStub) Stop(context.Context) error  { s.stops++; return s.stopErr }
func TestMaintenanceOwnerRetainsCommittedOutcomeWhenShutdownIsIncomplete(t *testing.T) {
	pending := &transaction.ShutdownIncomplete{ActiveSends: 1}
	owner := &maintenanceOwnerStub{stopErr: pending}
	called := 0
	err := runMaintenanceOwner(context.Background(), nil, owner, func(event.Stager) error { called++; return nil })
	require.Equal(t, 1, called)
	require.Equal(t, 1, owner.stops)
	require.ErrorIs(t, err, transaction.ErrShutdownIncomplete)
	require.True(t, maintenanceBusinessSucceeded(err))
	failure := errors.New("business execution unknown")
	err = runMaintenanceOwner(context.Background(), nil, owner, func(event.Stager) error { return failure })
	require.ErrorIs(t, err, failure)
	require.False(t, maintenanceBusinessSucceeded(err))
}
func TestMaintenanceStartupFailureDoesNotRunBusinessAndRetainsCleanupCounts(t *testing.T) {
	pending := &transaction.ShutdownIncomplete{RunningChecks: 1}
	startup := errors.New("private driver error")
	owner := &maintenanceOwnerStub{startErr: startup, stopErr: pending}
	err := runMaintenanceOwner(context.Background(), nil, owner, func(event.Stager) error { t.Fatal("business admitted after failed startup"); return nil })
	require.ErrorIs(t, err, startup)
	require.ErrorIs(t, err, transaction.ErrShutdownIncomplete)
	require.Equal(t, 1, owner.stops)
	require.NotContains(t, err.Error(), "private driver error")
}
func TestMaintenanceTransactionConfigIsExplicitAndCannotFallBack(t *testing.T) {
	_, err := loadMaintenanceMessaging("")
	require.Error(t, err)
	path := filepath.Join(t.TempDir(), "messaging.json")
	config := options.MessagingOptions{Provider: "nsq", PublishEnabled: true}
	raw, err := json.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0600))
	_, err = loadMaintenanceMessaging(path)
	require.Error(t, err)
	require.NoError(t, os.WriteFile(path, []byte(`{"provider":"rocketmq","unknown":"private-value"}`), 0600))
	_, err = loadMaintenanceMessaging(path)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "private-value")
	err = withMaintenanceStager(context.Background(), nil, nil, "standard", path, func(event.Stager) error { t.Fatal("ignored transaction config"); return nil })
	require.ErrorContains(t, err, "publishing profiles are retired")
}

type maintenancePoolStub struct{ closed int }

func (p *maintenancePoolStub) Close() error { p.closed++; return nil }
func TestMaintenancePoolRemainsOwnedUntilRealProtocolDrain(t *testing.T) {
	pool := &maintenancePoolStub{}
	closeMaintenancePool(pool, &maintenanceCompletionError{ShutdownErr: &transaction.ShutdownIncomplete{RunningChecks: 1}})
	require.Zero(t, pool.closed)
	closeMaintenancePool(pool, nil)
	require.Equal(t, 1, pool.closed)
}
