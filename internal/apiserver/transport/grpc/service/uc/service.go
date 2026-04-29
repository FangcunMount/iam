package uc

import (
	"google.golang.org/grpc"

	"github.com/FangcunMount/iam/internal/apiserver/transport/grpc/service/uc/identity"
)

// Service 聚合 UC 模块的所有 gRPC 服务
type Service struct {
	identity *identity.Service
}

// NewService 创建 UC gRPC 聚合服务
func NewService(identitySvc *identity.Service) *Service {
	return &Service{
		identity: identitySvc,
	}
}

// Register 注册所有 gRPC 服务
func (s *Service) Register(server *grpc.Server) {
	if s == nil || server == nil {
		return
	}

	// 注册 identity 相关服务
	if s.identity != nil {
		s.identity.RegisterService(server)
	}
}
