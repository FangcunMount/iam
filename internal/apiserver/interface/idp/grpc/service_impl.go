package grpc

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	idpv1 "github.com/FangcunMount/iam/api/grpc/iam/idp/v1"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
)

// GetWechatApp 查询微信应用
func (s *idpServer) GetWechatApp(ctx context.Context, req *idpv1.GetWechatAppRequest) (*idpv1.GetWechatAppResponse, error) {
	if req == nil || strings.TrimSpace(req.GetAppId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "app_id is required")
	}

	// 直接查询领域对象以获取完整信息（包括加密的 appSecret）
	app, err := s.wechatAppRepo.GetByAppID(ctx, req.GetAppId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	if app == nil {
		return nil, status.Error(codes.NotFound, "wechat app not found")
	}

	// 转换为 proto 消息（包含解密后的 appSecret）
	protoApp, err := wechatAppDomainToProto(ctx, app, s.secretVault)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to decrypt app secret: "+err.Error())
	}

	return &idpv1.GetWechatAppResponse{
		App: protoApp,
	}, nil
}

// wechatAppDomainToProto 将领域对象转换为 proto 消息（包含解密后的 appSecret）
func wechatAppDomainToProto(ctx context.Context, app *domain.WechatApp, secretVault domain.SecretVault) (*idpv1.WechatApp, error) {
	if app == nil {
		return nil, nil
	}

	protoApp := &idpv1.WechatApp{
		Id:     app.ID.String(),
		AppId:  app.AppID,
		Name:   app.Name,
		Type:   appTypeToProto(app.Type),
		Status: statusToProto(app.Status),
	}

	// 解密 appSecret
	if app.Cred != nil && app.Cred.Auth != nil && len(app.Cred.Auth.AppSecretCipher) > 0 {
		if secretVault != nil {
			plainSecret, err := secretVault.Decrypt(ctx, app.Cred.Auth.AppSecretCipher)
			if err != nil {
				return nil, err
			}
			protoApp.AppSecret = string(plainSecret)
		}
	}

	return protoApp, nil
}

// appTypeToProto 将领域 AppType 转换为 proto 枚举
func appTypeToProto(t domain.AppType) idpv1.WechatAppType {
	switch t {
	case domain.MiniProgram:
		return idpv1.WechatAppType_WECHAT_APP_TYPE_MINI_PROGRAM
	case domain.MP:
		return idpv1.WechatAppType_WECHAT_APP_TYPE_MP
	default:
		return idpv1.WechatAppType_WECHAT_APP_TYPE_UNSPECIFIED
	}
}

// statusToProto 将领域 Status 转换为 proto 枚举
func statusToProto(s domain.Status) idpv1.WechatAppStatus {
	switch s {
	case domain.StatusEnabled:
		return idpv1.WechatAppStatus_WECHAT_APP_STATUS_ENABLED
	case domain.StatusDisabled:
		return idpv1.WechatAppStatus_WECHAT_APP_STATUS_DISABLED
	case domain.StatusArchived:
		return idpv1.WechatAppStatus_WECHAT_APP_STATUS_ARCHIVED
	default:
		return idpv1.WechatAppStatus_WECHAT_APP_STATUS_UNSPECIFIED
	}
}

// toGRPCError 将应用层错误转换为 gRPC 错误
func toGRPCError(err error) error {
	if err == nil {
		return nil
	}

	// 简单的错误转换，可以根据实际需求扩展
	if strings.Contains(err.Error(), "not found") {
		return status.Error(codes.NotFound, err.Error())
	}

	return status.Error(codes.Internal, err.Error())
}
