package identity

import (
	iamgrpc "github.com/FangcunMount/iam/v3/internal/pkg/grpc"
)

func toGRPCError(err error) error {
	return iamgrpc.ToStatusError(err)
}
