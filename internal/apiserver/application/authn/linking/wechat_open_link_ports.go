package linking

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// WechatOpenLinkState 绑定场景 OAuth state 的创建结果（linking 本地端口类型）。
type WechatOpenLinkState struct {
	State     string
	Nonce     string
	ExpiresAt time.Time
}

// WechatOpenLinkContext 消费绑定场景 OAuth state 后恢复的上下文（linking 本地端口类型）。
type WechatOpenLinkContext struct {
	AppID  string
	UserID meta.ID
}

// WechatOpenLinkStateStarter 创建绑定场景 OAuth state（携带 user_id）。
//
// 这是 linking 包的本地端口；具体实现（如 challenge 用例）由装配层适配，
// 避免 linking 直接依赖 challenge 包。
type WechatOpenLinkStateStarter interface {
	StartWechatOpenLink(ctx context.Context, appID, redirectURI string, userID meta.ID, nonce string) (WechatOpenLinkState, error)
}

// WechatOpenLinkStateVerifier 校验并一次性消费绑定场景 OAuth state。
type WechatOpenLinkStateVerifier interface {
	VerifyAndConsumeWechatOpenLink(ctx context.Context, state string) (WechatOpenLinkContext, error)
}
