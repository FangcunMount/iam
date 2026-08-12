package wechatapp

import (
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// WechatApp 微信应用领域对象
type WechatApp struct {
	ID meta.ID

	AppID  string
	Name   string
	Type   AppType
	Status Status

	Cred *Credentials
}

// NewWechatApp 创建新的微信应用领域对象
func NewWechatApp(t AppType, aid string, opts ...WechatAppOption) *WechatApp {
	app := &WechatApp{
		Type:  t,
		AppID: aid,
	}

	for _, opt := range opts {
		opt(app)
	}

	return app
}

// WechatAppOption 微信应用选项
type WechatAppOption func(*WechatApp)

func WithWechatAppID(id meta.ID) WechatAppOption    { return func(w *WechatApp) { w.ID = id } }
func WithWechatAppName(name string) WechatAppOption { return func(w *WechatApp) { w.Name = name } }
func WithWechatAppStatus(status Status) WechatAppOption {
	return func(w *WechatApp) { w.Status = status }
}

// 状态检查方法
func (w *WechatApp) IsEnabled() bool  { return w.Status == StatusEnabled }
func (w *WechatApp) IsDisabled() bool { return w.Status == StatusDisabled }
func (w *WechatApp) IsArchived() bool { return w.Status == StatusArchived }

// 状态变更方法
func (w *WechatApp) Enable()  { w.Status = StatusEnabled }
func (w *WechatApp) Disable() { w.Status = StatusDisabled }
func (w *WechatApp) Archive() { w.Status = StatusArchived }
