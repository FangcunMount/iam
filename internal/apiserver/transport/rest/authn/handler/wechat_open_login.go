package handler

import (
	"errors"
	"io"

	"github.com/gin-gonic/gin"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin"
	req "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authn/request"
	resp "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authn/response"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// WechatOpenLoginAuthorizeHandler 暴露微信开放平台扫码登录的公开授权端点。
//
// 该端点面向未登录用户：服务端签发 OAuth state（CSRF 防护），并返回扫码授权 URL。
// AppID/RedirectURI 由服务端配置注入，前端不参与。
type WechatOpenLoginAuthorizeHandler struct {
	*BaseHandler
	authorize   *signin.StartWechatOpenAuthorize
	appID       string
	redirectURI string
}

// NewWechatOpenLoginAuthorizeHandler 创建处理器。
func NewWechatOpenLoginAuthorizeHandler(authorize *signin.StartWechatOpenAuthorize, appID, redirectURI string) *WechatOpenLoginAuthorizeHandler {
	return &WechatOpenLoginAuthorizeHandler{
		BaseHandler: NewBaseHandler(),
		authorize:   authorize,
		appID:       appID,
		redirectURI: redirectURI,
	}
}

// StartAuthorize 发起微信开放平台扫码登录授权，返回授权地址与 state。
// @Summary 发起微信开放平台扫码登录授权
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body req.LinkWechatOpenAuthorizeRequest false "可选 nonce"
// @Success 200 {object} resp.WechatOpenAuthorizeResponse "授权地址与 state"
// @Router /authn/wechat-open/authorize [post]
func (h *WechatOpenLoginAuthorizeHandler) StartAuthorize(c *gin.Context) {
	if h.authorize == nil {
		h.Error(c, perrors.WithCode(code.ErrInvalidArgument, "wechat open login is not configured"))
		return
	}
	var body req.LinkWechatOpenAuthorizeRequest
	if err := c.ShouldBindJSON(&body); err != nil && !errors.Is(err, io.EOF) {
		h.Error(c, perrors.WithCode(code.ErrBind, "invalid request body: %v", err))
		return
	}
	result, err := h.authorize.Execute(c.Request.Context(), signin.StartWechatOpenAuthorizeInput{
		AppID:       h.appID,
		RedirectURI: h.redirectURI,
		Nonce:       body.Nonce,
	})
	if err != nil {
		h.Error(c, err)
		return
	}
	h.Success(c, resp.WechatOpenAuthorizeResponse{
		State:        result.State,
		Nonce:        result.Nonce,
		AppID:        h.appID,
		AuthorizeURL: result.AuthorizeURL,
		ExpiresAt:    result.ExpiresAt,
	})
}
