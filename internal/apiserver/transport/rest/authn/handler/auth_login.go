package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/session"
	req "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authn/request"
	resp "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authn/response"
)

var _ *resp.TokenPair

// LoginV2 显式登录端点。
// @Summary 显式登录
// @Description 使用 auth_method 明确选择认证方式，method_payload 按认证方式解析。v2 只开放 password、phone_otp、wechat、wecom。
// @Tags Authentication-Auth
// @Accept json
// @Produce json
// @Param request body req.LoginV2Request true "显式登录请求"
// @Success 200 {object} resp.TokenPair "登录成功，返回访问令牌和刷新令牌"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 401 {object} map[string]interface{} "认证失败"
// @Router /authn/login [post]
func (h *AuthHandler) LoginV2(c *gin.Context) {
	var reqBody req.LoginV2Request
	if err := h.BindJSON(c, &reqBody); err != nil {
		h.Error(c, err)
		return
	}

	if err := reqBody.Validate(); err != nil {
		h.Error(c, err)
		return
	}

	loginReq, err := session.BuildExplicitLoginRequest(reqBody.AuthMethod, reqBody.MethodPayload)
	if err != nil {
		h.Error(c, err)
		return
	}
	loginReq.RemoteIP = c.ClientIP()
	loginReq.UserAgent = c.Request.UserAgent()
	h.executeLogin(c, loginReq)
}

// executeLogin 执行登录并返回令牌
func (h *AuthHandler) executeLogin(c *gin.Context, loginReq session.LoginRequest) {
	result, err := h.sessionService.Login(c.Request.Context(), loginReq)
	if err != nil {
		h.Error(c, err)
		return
	}

	// 转换令牌
	tokenPair := h.convertTokenPair(result.TokenPair)
	h.Success(c, tokenPair)
}
