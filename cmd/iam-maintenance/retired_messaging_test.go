package main

import (
	"context"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/stretchr/testify/require"
	"io"
	"testing"
)

func TestRetiredMessagingToolsRejectBeforeEnvironmentOrResourceUse(t *testing.T) {
	for _, args := range [][]string{
		{"m3-messaging", "--apply", "--plan", "missing-private-plan"},
		{"m3-messaging-recovery", "--restore", "--protection", "missing-private-plan"},
		{"reliable-messaging", "preflight", "--event-catalog", "missing-private-catalog"},
	} {
		t.Run(args[0], func(t *testing.T) {
			err := run(args, io.Discard)
			require.EqualError(t, err, "historical messaging cutover tools are retired; use current transaction diagnostics and retain original evidence")
		})
	}
}

func TestMaintenanceRejectsPublishingProfilesBeforeDatabaseOrConfig(t *testing.T) {
	for _, mode := range []string{"", "legacy", "standard", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			err := withMaintenanceStager(context.Background(), nil, nil, mode, "missing-private-config", func(event.Stager) error { t.Fatal("retired publishing mode admitted"); return nil })
			require.EqualError(t, err, "transaction maintenance is required; legacy and standard publishing profiles are retired")
		})
	}
}
