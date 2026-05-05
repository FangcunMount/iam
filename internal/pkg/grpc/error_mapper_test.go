package grpc

import (
	"errors"
	"net/http"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCodeForHTTPStatusMapsIAMTaxonomy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		httpStatus int
		want       codes.Code
	}{
		{name: "bad request", httpStatus: http.StatusBadRequest, want: codes.InvalidArgument},
		{name: "unauthorized", httpStatus: http.StatusUnauthorized, want: codes.Unauthenticated},
		{name: "forbidden", httpStatus: http.StatusForbidden, want: codes.PermissionDenied},
		{name: "not found", httpStatus: http.StatusNotFound, want: codes.NotFound},
		{name: "conflict", httpStatus: http.StatusConflict, want: codes.AlreadyExists},
		{name: "locked", httpStatus: http.StatusLocked, want: codes.FailedPrecondition},
		{name: "rate limited", httpStatus: http.StatusTooManyRequests, want: codes.ResourceExhausted},
		{name: "internal", httpStatus: http.StatusInternalServerError, want: codes.Internal},
		{name: "bad gateway", httpStatus: http.StatusBadGateway, want: codes.Unavailable},
		{name: "unavailable", httpStatus: http.StatusServiceUnavailable, want: codes.Unavailable},
		{name: "gateway timeout", httpStatus: http.StatusGatewayTimeout, want: codes.DeadlineExceeded},
		{name: "unknown 4xx", httpStatus: http.StatusTeapot, want: codes.Unknown},
		{name: "unknown 5xx", httpStatus: 599, want: codes.Internal},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, CodeForHTTPStatus(tt.httpStatus))
		})
	}
}

func TestToStatusErrorUsesRegisteredCoderHTTPStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want codes.Code
	}{
		{name: "invalid argument", err: perrors.WithCode(code.ErrInvalidArgument, "invalid"), want: codes.InvalidArgument},
		{name: "unauthenticated", err: perrors.WithCode(code.ErrTokenInvalid, "invalid token"), want: codes.Unauthenticated},
		{name: "permission denied", err: perrors.WithCode(code.ErrPermissionDenied, "denied"), want: codes.PermissionDenied},
		{name: "not found", err: perrors.WithCode(code.ErrWechatAppNotFound, "missing"), want: codes.NotFound},
		{name: "already exists", err: perrors.WithCode(code.ErrRoleAlreadyExists, "exists"), want: codes.AlreadyExists},
		{name: "locked", err: perrors.WithCode(code.ErrCredentialLocked, "locked"), want: codes.FailedPrecondition},
		{name: "rate limited", err: perrors.WithCode(code.ErrOTPSendTooFrequent, "too frequent"), want: codes.ResourceExhausted},
		{name: "bad gateway", err: perrors.WithCode(code.ErrIDPExchangeFailed, "idp failed"), want: codes.Unavailable},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, status.Code(ToStatusError(tt.err)))
		})
	}
}

func TestToStatusErrorPreservesStatusAndDefaultsUnknownErrorsToInternal(t *testing.T) {
	t.Parallel()

	require.Equal(t, codes.Unimplemented, status.Code(ToStatusError(status.Error(codes.Unimplemented, "not implemented"))))
	require.Equal(t, codes.Internal, status.Code(ToStatusError(errors.New("plain error"))))
	require.NoError(t, ToStatusError(nil))
}
