package linking

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// CompleteWechatOpenLinkInput 处理微信开放平台扫码绑定回调。
//
// ExpectedUserID 由 transport 从已验证 access token 注入，用于确认 state 归属调用方本人
// （基本的归属/CSRF 校验，与产品准入边界无关）。零值表示跳过该校验。
type CompleteWechatOpenLinkInput struct {
	State          string
	Code           string
	ExpectedUserID meta.ID
}

// CompleteWechatOpenLink 消费绑定场景 OAuth state，并把微信开放平台身份绑定到 state 记录的用户。
//
// 通用能力：只做绑定，不做"是否 operating 用户"之类产品检查（由调用方在自己页面控制入口）。
type CompleteWechatOpenLink struct {
	states WechatOpenLinkStateVerifier
	linker Linker
}

// NewCompleteWechatOpenLink 创建用例。
func NewCompleteWechatOpenLink(
	states WechatOpenLinkStateVerifier,
	linker Linker,
) *CompleteWechatOpenLink {
	return &CompleteWechatOpenLink{states: states, linker: linker}
}

// Execute 执行绑定回调。
func (u *CompleteWechatOpenLink) Execute(ctx context.Context, input CompleteWechatOpenLinkInput) (*LinkResult, error) {
	if u == nil || u.states == nil || u.linker == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "complete wechat open link use case is not initialized")
	}
	if strings.TrimSpace(input.Code) == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "code is required")
	}

	stateCtx, err := u.states.VerifyAndConsumeWechatOpenLink(ctx, input.State)
	if err != nil {
		return nil, err
	}
	if stateCtx.UserID.IsZero() {
		return nil, perrors.WithCode(code.ErrStateMismatch, "link state must carry user_id")
	}
	if !input.ExpectedUserID.IsZero() && input.ExpectedUserID != stateCtx.UserID {
		return nil, perrors.WithCode(code.ErrStateMismatch, "link state does not belong to the current user")
	}

	return u.linker.Link(ctx, LinkRequest{
		UserID: stateCtx.UserID,
		Input: LinkWechatOpenInput{
			AppID: stateCtx.AppID,
			Code:  strings.TrimSpace(input.Code),
		},
	})
}
