package handler

import (
	"github.com/gin-gonic/gin"

	req "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/request"
	resp "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/response"
)

// PreparePhoneOTPLogin 登录预准备：写入 Redis 后发布 OTP（sms.provider=mq 走 NSQ；log 仅打日志）
// @Summary 登录预准备-发送手机验证码
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body req.PreparePhoneOTPLoginRequest true "手机号"
// @Success 200 {object} resp.MessageResponse "已受理"
// @Router /authn/login/prep/phone-otp [post]
func (h *AuthHandler) PreparePhoneOTPLogin(c *gin.Context) {
	var reqBody req.PreparePhoneOTPLoginRequest
	if err := h.BindJSON(c, &reqBody); err != nil {
		h.Error(c, err)
		return
	}
	if err := reqBody.Validate(); err != nil {
		h.Error(c, err)
		return
	}
	if err := h.loginPreparation.SendPhoneOTPForLogin(c.Request.Context(), reqBody.Phone); err != nil {
		h.Error(c, err)
		return
	}
	h.Success(c, resp.MessageResponse{Message: "verification code sent"})
}
