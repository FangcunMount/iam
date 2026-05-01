package idp

import (
	"google.golang.org/grpc"

	idpv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/idp/v2"
	wechatappApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/idp/wechatapp"
	wechatappDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
)

// Service IDP gRPC 服务
type Service struct {
	idpService idpServer
}

// Register 注册 gRPC 服务到 gRPC 服务器（与 UserModule 的接口保持一致）
func (s *Service) Register(server *grpc.Server) {
	s.RegisterService(server)
}

// NewService 创建 IDP gRPC 服务
func NewService(
	wechatAppService wechatappApp.WechatAppApplicationService,
	wechatAppRepo wechatappDomain.Repository,
	secretVault wechatappDomain.SecretVault,
) *Service {
	return &Service{
		idpService: idpServer{
			wechatAppService: wechatAppService,
			wechatAppRepo:    wechatAppRepo,
			secretVault:      secretVault,
		},
	}
}

// RegisterService 注册 gRPC 服务到 gRPC 服务器
func (s *Service) RegisterService(server *grpc.Server) {
	idpv2.RegisterIDPServiceServer(server, &s.idpService)
}

// idpServer IDP 服务实现
type idpServer struct {
	idpv2.UnimplementedIDPServiceServer
	wechatAppService wechatappApp.WechatAppApplicationService
	wechatAppRepo    wechatappDomain.Repository
	secretVault      wechatappDomain.SecretVault
}
