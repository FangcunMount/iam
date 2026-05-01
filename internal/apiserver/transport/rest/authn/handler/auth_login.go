package handler

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login"
	req "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authn/request"
	resp "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authn/response"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

var _ *resp.TokenPair

type loginPayloadAdapter func(json.RawMessage) (login.LoginRequest, error)

var loginPayloadAdapters = map[string]loginPayloadAdapter{
	"password":  passwordLoginRequest,
	"phone_otp": phoneOTPLoginRequest,
	"wechat":    wechatLoginRequest,
	"wecom":     wecomLoginRequest,
}

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

	loginReq, err := buildLoginRequest(reqBody.AuthMethod, reqBody.MethodPayload)
	if err != nil {
		h.Error(c, err)
		return
	}
	h.executeLogin(c, loginReq)
}

func buildLoginRequest(method string, payload json.RawMessage) (login.LoginRequest, error) {
	adapter, ok := loginPayloadAdapters[method]
	if !ok {
		return login.LoginRequest{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication method: %s", method)
	}
	return adapter(payload)
}

func passwordLoginRequest(payload json.RawMessage) (login.LoginRequest, error) {
	var creds req.PasswordCredentials
	if err := json.Unmarshal(payload, &creds); err != nil {
		return login.LoginRequest{}, perrors.WithCode(code.ErrBind, "invalid password method_payload: %v", err)
	}

	loginReq := login.LoginRequest{
		SelectionMode: login.SignInSelectionExplicit,
		AuthType:      login.AuthTypePassword,
		Username:      &creds.Username,
		Password:      &creds.Password,
	}
	if creds.TenantID != 0 {
		loginReq.TenantID = meta.FromUint64(creds.TenantID)
	}

	return loginReq, nil
}

func phoneOTPLoginRequest(payload json.RawMessage) (login.LoginRequest, error) {
	var creds req.PhoneOTPCredentials
	if err := json.Unmarshal(payload, &creds); err != nil {
		return login.LoginRequest{}, perrors.WithCode(code.ErrBind, "invalid phone OTP method_payload: %v", err)
	}

	return login.LoginRequest{
		SelectionMode: login.SignInSelectionExplicit,
		AuthType:      login.AuthTypePhoneOTP,
		PhoneE164:     &creds.Phone,
		OTPCode:       &creds.OTPCode,
	}, nil
}

func wechatLoginRequest(payload json.RawMessage) (login.LoginRequest, error) {
	var creds req.WeChatCredentials
	if err := json.Unmarshal(payload, &creds); err != nil {
		return login.LoginRequest{}, perrors.WithCode(code.ErrBind, "invalid wechat method_payload: %v", err)
	}

	return login.LoginRequest{
		SelectionMode: login.SignInSelectionExplicit,
		AuthType:      login.AuthTypeWechat,
		WechatAppID:   &creds.AppID,
		WechatJSCode:  &creds.Code,
	}, nil
}

func wecomLoginRequest(payload json.RawMessage) (login.LoginRequest, error) {
	var creds req.WeComCredentials
	if err := json.Unmarshal(payload, &creds); err != nil {
		return login.LoginRequest{}, perrors.WithCode(code.ErrBind, "invalid wecom method_payload: %v", err)
	}

	return login.LoginRequest{
		SelectionMode: login.SignInSelectionExplicit,
		AuthType:      login.AuthTypeWecom,
		WecomCorpID:   &creds.CorpID,
		WecomCode:     &creds.AuthCode,
	}, nil
}

// executeLogin 执行登录并返回令牌
func (h *AuthHandler) executeLogin(c *gin.Context, loginReq login.LoginRequest) {
	result, err := h.loginService.Login(c.Request.Context(), loginReq)
	if err != nil {
		h.Error(c, err)
		return
	}

	tokenPair := h.convertTokenPair(result.TokenPair)
	h.Success(c, tokenPair)
}
