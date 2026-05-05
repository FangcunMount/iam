package grpc

import (
	"net/http"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CodeForHTTPStatus maps IAM's registered HTTP-oriented error taxonomy to gRPC.
func CodeForHTTPStatus(httpStatus int) codes.Code {
	switch httpStatus {
	case http.StatusBadRequest:
		return codes.InvalidArgument
	case http.StatusUnauthorized:
		return codes.Unauthenticated
	case http.StatusForbidden:
		return codes.PermissionDenied
	case http.StatusNotFound:
		return codes.NotFound
	case http.StatusConflict:
		return codes.AlreadyExists
	case http.StatusPreconditionFailed, http.StatusLocked:
		return codes.FailedPrecondition
	case http.StatusRequestedRangeNotSatisfiable:
		return codes.OutOfRange
	case http.StatusTooManyRequests:
		return codes.ResourceExhausted
	case 499:
		return codes.Canceled
	case http.StatusNotImplemented:
		return codes.Unimplemented
	case http.StatusBadGateway, http.StatusServiceUnavailable:
		return codes.Unavailable
	case http.StatusGatewayTimeout:
		return codes.DeadlineExceeded
	case http.StatusInternalServerError:
		return codes.Internal
	default:
		if httpStatus >= 500 {
			return codes.Internal
		}
		return codes.Unknown
	}
}

// CodedStatusError converts a component-base coded error into a gRPC status.
func CodedStatusError(err error) (error, bool) {
	if err == nil {
		return nil, false
	}
	coder := perrors.ParseCoder(err)
	if coder == nil {
		return nil, false
	}
	return status.Error(CodeForHTTPStatus(coder.HTTPStatus()), coder.String()), true
}

// ToStatusError converts application errors to transport-level gRPC errors.
func ToStatusError(err error) error {
	if err == nil {
		return nil
	}
	if st, ok := status.FromError(err); ok {
		return st.Err()
	}
	if coded, ok := CodedStatusError(err); ok {
		return coded
	}
	return status.Error(codes.Internal, err.Error())
}
