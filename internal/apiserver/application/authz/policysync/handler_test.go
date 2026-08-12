package policysync

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/eventing"
	"github.com/stretchr/testify/require"
)

func TestHandlerReloadsRuntimeAndRecordsVersionEvent(t *testing.T) {
	t.Parallel()

	reloader := &reloaderStub{}
	recorder := &recorderStub{}
	handler := NewHandler(reloader, recorder)

	err := handler.Handle(context.Background(), []byte(`{"tenant_id":"tenant-a","version":7}`), eventing.AuthzVersionChanged)

	require.NoError(t, err)
	require.Equal(t, 1, reloader.reloads)
	require.Equal(t, "tenant-a", recorder.tenantID)
	require.Equal(t, int64(7), recorder.version)
	require.False(t, recorder.eventAt.IsZero())
}

func TestHandlerIgnoresOtherEvents(t *testing.T) {
	t.Parallel()

	reloader := &reloaderStub{}
	handler := NewHandler(reloader, nil)

	err := handler.Handle(context.Background(), []byte(`{"tenant_id":"tenant-a","version":7}`), "iam.login_otp_sms")

	require.NoError(t, err)
	require.Zero(t, reloader.reloads)
}

func TestHandlerRejectsInvalidPayload(t *testing.T) {
	t.Parallel()

	reloader := &reloaderStub{}
	handler := NewHandler(reloader, nil)

	err := handler.Handle(context.Background(), []byte(`{}`), eventing.AuthzVersionChanged)

	require.Error(t, err)
	require.Zero(t, reloader.reloads)
}

func TestInstanceChannelIsEphemeralAndInstanceScoped(t *testing.T) {
	t.Parallel()

	first := InstanceChannel("iam-host-1", 1001)
	second := InstanceChannel("iam-host-2", 1002)

	require.Equal(t, "iam-policy-sync.iam-host-1.1001#ephemeral", first)
	require.Equal(t, "iam-policy-sync.iam-host-2.1002#ephemeral", second)
	require.NotEqual(t, first, second)
}

func TestInstanceChannelSanitizesHostName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "iam-policy-sync.iam-host.1001#ephemeral", InstanceChannel(" iam/host ", 1001))
}

type reloaderStub struct {
	reloads int
}

func (s *reloaderStub) LoadPolicy(context.Context) error {
	s.reloads++
	return nil
}

type recorderStub struct {
	tenantID string
	version  int64
	eventAt  time.Time
}

func (s *recorderStub) RecordPolicyVersionEvent(tenantID string, version int64, eventAt time.Time) {
	s.tenantID = tenantID
	s.version = version
	s.eventAt = eventAt
}
