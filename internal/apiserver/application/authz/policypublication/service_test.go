package policypublication

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/eventing"
	"github.com/stretchr/testify/require"
)

func TestHandlerReloadsRuntimeAndRecordsVersionEvent(t *testing.T) {
	t.Parallel()

	reloader := &reloaderStub{}
	recorder := &recorderStub{}
	handler := NewService(reloader, recorder)

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
	handler := NewService(reloader, nil)

	err := handler.Handle(context.Background(), []byte(`{"tenant_id":"tenant-a","version":7}`), "iam.login_otp_sms")

	require.NoError(t, err)
	require.Zero(t, reloader.reloads)
}

func TestHandlerRejectsInvalidPayload(t *testing.T) {
	t.Parallel()

	reloader := &reloaderStub{}
	handler := NewService(reloader, nil)

	err := handler.Handle(context.Background(), []byte(`{}`), eventing.AuthzVersionChanged)

	require.Error(t, err)
	require.Zero(t, reloader.reloads)
}

func TestHandlerReturnsReloadFailureForMessageRetry(t *testing.T) {
	t.Parallel()

	reloader := &reloaderStub{err: errors.New("database unavailable")}
	handler := NewService(reloader, nil)

	err := handler.Handle(context.Background(), []byte(`{"tenant_id":"tenant-a","version":7}`), eventing.AuthzVersionChanged)

	require.ErrorContains(t, err, "reload authz runtime policy")
	require.Equal(t, 3, reloader.reloads)
}

type reloaderStub struct {
	reloads int
	err     error
}

func (s *reloaderStub) LoadPolicy(context.Context) error {
	s.reloads++
	return s.err
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
