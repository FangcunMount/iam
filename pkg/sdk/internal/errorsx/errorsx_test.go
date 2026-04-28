package errorsx

import (
	"testing"
	"time"

	sdkerrors "github.com/FangcunMount/iam/pkg/sdk/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAnalyzeResourceExhaustedProducesRateLimitDetails(t *testing.T) {
	t.Parallel()

	details := Analyze(status.Error(codes.ResourceExhausted, "slow down"))
	require.NotNil(t, details)
	require.Equal(t, CategoryRateLimit, details.Category)
	require.True(t, details.IsRetryable)
	require.Equal(t, ActionRateLimit, details.SuggestedAction)
	require.Equal(t, 5*time.Second, details.RetryAfter)
}

func TestMatchersAndHandlersRemainUsable(t *testing.T) {
	t.Parallel()

	notFound := status.Error(codes.NotFound, "missing")
	require.True(t, ResourceErrors.Match(notFound))
	require.False(t, AuthErrors.Match(notFound))

	handler := NewErrorHandler().OnNotFound(func(err error) error { return nil })
	require.NoError(t, handler.Handle(notFound))
	require.NoError(t, IgnoreAlreadyExists(status.Error(codes.AlreadyExists, "dup")))
}

func TestAnalyzePrefersIAMErrorCode(t *testing.T) {
	t.Parallel()

	details := Analyze(sdkerrors.WrapWithCode(status.Error(codes.PermissionDenied, "denied"), "auth.permission_denied", "permission denied"))
	require.Equal(t, "auth.permission_denied", details.Code)
	require.Equal(t, CategoryAuth, details.Category)
}
