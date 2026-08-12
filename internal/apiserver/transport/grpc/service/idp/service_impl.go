package idp

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	idpv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/idp/v2"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	iamgrpc "github.com/FangcunMount/iam/v3/internal/pkg/grpc"
)

// GetWechatApp 查询微信应用
func (s *idpServer) GetWechatApp(ctx context.Context, req *idpv2.GetWechatAppRequest) (*idpv2.GetWechatAppResponse, error) {
	if req == nil || strings.TrimSpace(req.GetAppId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "app_id is required")
	}

	// 直接查询领域对象以获取完整信息（包括加密的 appSecret）
	app, err := s.wechatAppRepo.GetByAppID(ctx, req.GetAppId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	if app == nil {
		return nil, toGRPCError(perrors.WithCode(code.ErrWechatAppNotFound, "wechat app not found: %s", req.GetAppId()))
	}

	// 转换为 proto 消息（包含解密后的 appSecret）
	protoApp, err := wechatAppDomainToProto(ctx, app, s.secretVault)
	if err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}

	return &idpv2.GetWechatAppResponse{
		App: protoApp,
	}, nil
}

// GetWechatAccessToken 获取微信应用访问令牌。
func (s *idpServer) GetWechatAccessToken(ctx context.Context, req *idpv2.GetWechatAccessTokenRequest) (*idpv2.GetWechatAccessTokenResponse, error) {
	if s.wechatAppTokenService == nil {
		return nil, status.Error(codes.Unimplemented, "wechat app token service not configured")
	}
	if req == nil || strings.TrimSpace(req.GetAppId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "app_id is required")
	}
	token, err := s.wechatAppTokenService.GetAccessToken(ctx, req.GetAppId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &idpv2.GetWechatAccessTokenResponse{AccessToken: token}, nil
}

// RefreshWechatAccessToken 强制刷新微信应用访问令牌。
func (s *idpServer) RefreshWechatAccessToken(ctx context.Context, req *idpv2.RefreshWechatAccessTokenRequest) (*idpv2.RefreshWechatAccessTokenResponse, error) {
	if s.wechatAppTokenService == nil {
		return nil, status.Error(codes.Unimplemented, "wechat app token service not configured")
	}
	if req == nil || strings.TrimSpace(req.GetAppId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "app_id is required")
	}
	token, err := s.wechatAppTokenService.RefreshAccessToken(ctx, req.GetAppId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &idpv2.RefreshWechatAccessTokenResponse{AccessToken: token}, nil
}

// wechatAppDomainToProto 将领域对象转换为 proto 消息（包含解密后的 appSecret）
func wechatAppDomainToProto(ctx context.Context, app *domain.WechatApp, secretVault domain.SecretVault) (*idpv2.WechatApp, error) {
	if app == nil {
		return nil, nil
	}

	protoApp := &idpv2.WechatApp{
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
func appTypeToProto(t domain.AppType) idpv2.WechatAppType {
	switch t {
	case domain.MiniProgram:
		return idpv2.WechatAppType_WECHAT_APP_TYPE_MINI_PROGRAM
	case domain.MP:
		return idpv2.WechatAppType_WECHAT_APP_TYPE_MP
	case domain.OpenPlatformWebsite:
		return idpv2.WechatAppType_WECHAT_APP_TYPE_OPEN_PLATFORM_WEBSITE
	default:
		return idpv2.WechatAppType_WECHAT_APP_TYPE_UNSPECIFIED
	}
}

// statusToProto 将领域 Status 转换为 proto 枚举
func statusToProto(s domain.Status) idpv2.WechatAppStatus {
	switch s {
	case domain.StatusEnabled:
		return idpv2.WechatAppStatus_WECHAT_APP_STATUS_ENABLED
	case domain.StatusDisabled:
		return idpv2.WechatAppStatus_WECHAT_APP_STATUS_DISABLED
	case domain.StatusArchived:
		return idpv2.WechatAppStatus_WECHAT_APP_STATUS_ARCHIVED
	default:
		return idpv2.WechatAppStatus_WECHAT_APP_STATUS_UNSPECIFIED
	}
}

// toGRPCError 将应用层错误转换为 gRPC 错误
func toGRPCError(err error) error {
	return iamgrpc.ToStatusError(err)
}
