package errors

import (
	stdErrors "errors"
	"net/http"
	"testing"

	iamgrpc "github.com/FangcunMount/iam/v3/internal/pkg/grpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWrapPreservesGRPCSemantics(t *testing.T) {
	t.Parallel()

	err := Wrap(status.Error(codes.NotFound, "user not found"))
	require.Error(t, err)
	require.True(t, IsNotFound(err))
	require.False(t, IsUnauthorized(err))
	require.Equal(t, codes.NotFound, GRPCCode(err))
	require.Equal(t, "user not found", Message(err))
	require.Equal(t, 404, ToHTTPStatus(err))

	iamErr, ok := AsIAMError(err)
	require.True(t, ok)
	require.Equal(t, "NotFound", iamErr.Code)
	require.Equal(t, codes.NotFound, iamErr.GRPCCode)
}

func TestWrapWithCodeKeepsCustomCodeAndRetryableStatus(t *testing.T) {
	t.Parallel()

	err := WrapWithCode(status.Error(codes.Unavailable, "downstream unavailable"), "UPSTREAM_DOWN", "iam upstream unavailable")
	require.Error(t, err)
	require.True(t, IsServiceUnavailable(err))
	require.True(t, IsRetryable(err))
	require.Equal(t, codes.Unavailable, GRPCCode(err))
	require.Equal(t, "iam upstream unavailable", Message(err))
	require.Equal(t, 503, ToHTTPStatus(err))

	iamErr, ok := AsIAMError(err)
	require.True(t, ok)
	require.Equal(t, "UPSTREAM_DOWN", iamErr.Code)
	require.Equal(t, "iam upstream unavailable", iamErr.Message)
}

func TestSDKErrorMappingTracksServerGRPCMapper(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		httpStatus int
		wantCode   codes.Code
		wantHTTP   int
		wantIs     error
	}{
		{name: "bad request", httpStatus: http.StatusBadRequest, wantCode: codes.InvalidArgument, wantHTTP: 400, wantIs: ErrInvalidArgument},
		{name: "unauthorized", httpStatus: http.StatusUnauthorized, wantCode: codes.Unauthenticated, wantHTTP: 401, wantIs: ErrUnauthorized},
		{name: "forbidden", httpStatus: http.StatusForbidden, wantCode: codes.PermissionDenied, wantHTTP: 403, wantIs: ErrPermissionDenied},
		{name: "not found", httpStatus: http.StatusNotFound, wantCode: codes.NotFound, wantHTTP: 404, wantIs: ErrNotFound},
		{name: "conflict", httpStatus: http.StatusConflict, wantCode: codes.AlreadyExists, wantHTTP: 409, wantIs: ErrAlreadyExists},
		{name: "locked", httpStatus: http.StatusLocked, wantCode: codes.FailedPrecondition, wantHTTP: 400},
		{name: "rate limited", httpStatus: http.StatusTooManyRequests, wantCode: codes.ResourceExhausted, wantHTTP: 429, wantIs: ErrRateLimited},
		{name: "internal", httpStatus: http.StatusInternalServerError, wantCode: codes.Internal, wantHTTP: 500, wantIs: ErrInternal},
		{name: "bad gateway", httpStatus: http.StatusBadGateway, wantCode: codes.Unavailable, wantHTTP: 503, wantIs: ErrServiceUnavailable},
		{name: "unavailable", httpStatus: http.StatusServiceUnavailable, wantCode: codes.Unavailable, wantHTTP: 503, wantIs: ErrServiceUnavailable},
		{name: "gateway timeout", httpStatus: http.StatusGatewayTimeout, wantCode: codes.DeadlineExceeded, wantHTTP: 504, wantIs: ErrTimeout},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Wrap(status.Error(iamgrpc.CodeForHTTPStatus(tt.httpStatus), tt.name))
			require.Equal(t, tt.wantCode, GRPCCode(err))
			require.Equal(t, tt.wantHTTP, ToHTTPStatus(err))
			if tt.wantIs != nil {
				require.True(t, stdErrors.Is(err, tt.wantIs))
			}
		})
	}
}
