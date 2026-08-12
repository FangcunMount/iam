package wechatapp

import domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/wechatapp"

func toWechatAppResult(app *domain.WechatApp) *WechatAppResult {
	if app == nil {
		return nil
	}
	return &WechatAppResult{
		ID:     app.ID.String(),
		AppID:  app.AppID,
		Name:   app.Name,
		Type:   app.Type,
		Status: app.Status,
	}
}
