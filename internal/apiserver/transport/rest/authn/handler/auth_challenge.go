package handler

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/gin-gonic/gin"

	req "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authn/request"
	resp "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authn/response"
)

// SendLoginPhoneOTP 发送登录短信验证码。
// @Summary 发送登录短信验证码
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body req.SendLoginPhoneOTPRequest true "手机号"
// @Success 200 {object} resp.MessageResponse "已受理"
// @Router /authn/challenges/phone-otp [post]
func (h *AuthHandler) SendLoginPhoneOTP(c *gin.Context) {
	var reqBody req.SendLoginPhoneOTPRequest
	if err := h.BindJSON(c, &reqBody); err != nil {
		h.Error(c, err)
		return
	}
	if err := reqBody.Validate(); err != nil {
		h.Error(c, err)
		return
	}
	if h.loginPhoneOTPSender == nil {
		h.Error(c, perrors.WithCode(code.ErrInvalidArgument, "login phone OTP is not configured"))
		return
	}
	if err := h.loginPhoneOTPSender.SendLoginPhoneOTP(c.Request.Context(), reqBody.Phone); err != nil {
		h.Error(c, err)
		return
	}
	h.Success(c, resp.MessageResponse{Message: "verification code sent"})
}
