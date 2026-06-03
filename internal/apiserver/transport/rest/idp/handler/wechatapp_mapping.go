package handler

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"

	"github.com/FangcunMount/iam/v2/internal/apiserver/application/idp/wechatapp"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/idp/request"
	"github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/idp/response"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

func createDTOFromRequest(req request.CreateWechatAppRequest) wechatapp.CreateWechatAppDTO {
	return wechatapp.CreateWechatAppDTO{
		AppID:     req.AppID,
		Name:      req.Name,
		Type:      domain.AppType(req.Type),
		AppSecret: req.AppSecret,
	}
}

func toWechatAppResponse(result *wechatapp.WechatAppResult) *response.WechatAppResponse {
	if result == nil {
		return nil
	}
	return &response.WechatAppResponse{
		ID:     result.ID,
		AppID:  result.AppID,
		Name:   result.Name,
		Type:   string(result.Type),
		Status: string(result.Status),
	}
}

func listFilterFromRequest(req request.ListWechatAppsRequest) (wechatapp.ListWechatAppsFilter, error) {
	filter := wechatapp.ListWechatAppsFilter{}

	if req.Type != "" {
		appType, err := parseAppType(req.Type)
		if err != nil {
			return filter, err
		}
		filter.Type = &appType
	}
	if req.Status != "" {
		status, err := parseAppStatus(req.Status)
		if err != nil {
			return filter, err
		}
		filter.Status = &status
	}

	return filter, nil
}

func updateDTOFromRequest(req request.UpdateWechatAppRequest) (wechatapp.UpdateWechatAppDTO, error) {
	dto := wechatapp.UpdateWechatAppDTO{
		Name: req.Name,
	}

	if req.Type != nil {
		appType, err := parseAppType(*req.Type)
		if err != nil {
			return dto, err
		}
		dto.Type = &appType
	}

	if dto.Name == nil && dto.Type == nil {
		return dto, perrors.WithCode(code.ErrInvalidArgument, "at least one field must be updated")
	}
	return dto, nil
}

func parseAppType(raw string) (domain.AppType, error) {
	switch domain.AppType(raw) {
	case domain.MiniProgram, domain.MP, domain.OpenPlatformWebsite:
		return domain.AppType(raw), nil
	default:
		return "", perrors.WithCode(code.ErrWechatAppTypeInvalid, "invalid wechat app type: %s", raw)
	}
}

func parseAppStatus(raw string) (domain.Status, error) {
	switch domain.Status(raw) {
	case domain.StatusEnabled, domain.StatusDisabled, domain.StatusArchived:
		return domain.Status(raw), nil
	default:
		return "", perrors.WithCode(code.ErrWechatAppStatusInvalid, "invalid wechat app status: %s", raw)
	}
}
