package identity

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/FangcunMount/component-base/pkg/errors"
)

func toGRPCError(err error) error {
	if err == nil {
		return nil
	}

	if coder := errors.ParseCoder(err); coder != nil {
		switch coder.HTTPStatus() {
		case 400:
			return status.Error(codes.InvalidArgument, coder.String())
		case 401:
			return status.Error(codes.Unauthenticated, coder.String())
		case 403:
			return status.Error(codes.PermissionDenied, coder.String())
		case 404:
			return status.Error(codes.NotFound, coder.String())
		case 409:
			return status.Error(codes.AlreadyExists, coder.String())
		case 429:
			return status.Error(codes.ResourceExhausted, coder.String())
		case 500:
			return status.Error(codes.Internal, coder.String())
		case 503:
			return status.Error(codes.Unavailable, coder.String())
		default:
			return status.Error(codes.Unknown, coder.String())
		}
	}

	return status.Error(codes.Internal, err.Error())
}
