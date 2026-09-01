package policychange

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReloadRuntimePolicyWithErrorRetriesAndReturnsFinalFailure(t *testing.T) {
	t.Parallel()

	reloader := &runtimePolicyReloaderStub{err: errors.New("load failed")}
	err := ReloadRuntimePolicyWithError(context.Background(), reloader, "test")

	require.ErrorContains(t, err, "load failed")
	require.Equal(t, 3, reloader.loads)
}

func TestReloadRuntimePolicyWithErrorStopsAfterSuccess(t *testing.T) {
	t.Parallel()

	reloader := &runtimePolicyReloaderStub{failuresBeforeSuccess: 1}
	err := ReloadRuntimePolicyWithError(context.Background(), reloader, "test")

	require.NoError(t, err)
	require.Equal(t, 2, reloader.loads)
}

type runtimePolicyReloaderStub struct {
	loads                 int
	failuresBeforeSuccess int
	err                   error
}

func (s *runtimePolicyReloaderStub) LoadPolicy(context.Context) error {
	s.loads++
	if s.loads <= s.failuresBeforeSuccess {
		return errors.New("transient load failure")
	}
	return s.err
}
