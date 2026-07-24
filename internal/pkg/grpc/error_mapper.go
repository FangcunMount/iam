package grpc

import (
	"context"
	"errors"
	"net/http"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const componentBaseUnknownErrorCode = 1

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
	// component-base maps every ordinary Go error to its internal fallback
	// coder (code 1). That fallback is not a registered IAM business error and
	// must use this transport's stable public Internal message.
	if coder == nil || coder.Code() == componentBaseUnknownErrorCode {
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
		return status.Error(st.Code(), publicMessageForStatus(st.Code(), st.Message()))
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, publicMessageForStatus(codes.Canceled, ""))
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, publicMessageForStatus(codes.DeadlineExceeded, ""))
	}
	if coded, ok := CodedStatusError(err); ok {
		return coded
	}
	return status.Error(codes.Internal, publicMessageForStatus(codes.Internal, ""))
}

// PublicStatusMessage returns the client-safe message used by ToStatusError.
// It is intended for batch response fields that cannot carry a gRPC status.
func PublicStatusMessage(err error) string {
	if err == nil {
		return ""
	}
	return status.Convert(ToStatusError(err)).Message()
}

func publicMessageForStatus(code codes.Code, message string) string {
	switch code {
	case codes.Internal, codes.Unknown, codes.DataLoss:
		return "internal server error"
	case codes.Unavailable:
		return "service unavailable"
	case codes.DeadlineExceeded:
		return "deadline exceeded"
	case codes.Canceled:
		return "request canceled"
	default:
		return message
	}
}
