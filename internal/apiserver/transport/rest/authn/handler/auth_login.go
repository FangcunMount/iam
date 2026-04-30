package handler

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/apiserver/application/authn/login"
	req "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/request"
	resp "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/response"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

var _ *resp.TokenPair

type loginPayloadAdapter func(json.RawMessage, login.SignInSelectionMode, string) (login.LoginRequest, error)

var loginPayloadAdapters = map[string]loginPayloadAdapter{
	"password":  passwordLoginRequest,
	"phone_otp": phoneOTPLoginRequest,
	"wechat":    wechatLoginRequest,
	"wecom":     wecomLoginRequest,
}

// Login 统一登录端点
// @Summary 用户登录
// @Description 支持多种登录方式：密码登录、手机验证码登录、微信小程序登录、企业微信登录；v1 使用 legacy inference 从 method/credentials 推断场景
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body req.LoginRequest true "登录请求"
// @Success 200 {object} resp.TokenPair "登录成功，返回访问令牌和刷新令牌"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 401 {object} map[string]interface{} "认证失败"
// @Router /authn/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var reqBody req.LoginRequest
	if err := h.BindJSON(c, &reqBody); err != nil {
		h.Error(c, err)
		return
	}

	if err := reqBody.Validate(); err != nil {
		h.Error(c, err)
		return
	}

	loginReq, err := buildLoginRequest(reqBody.Method, reqBody.Credentials, login.SignInSelectionLegacy, "credentials")
	if err != nil {
		h.Error(c, err)
		return
	}
	h.executeLogin(c, loginReq)
}

// LoginV2 显式登录端点。
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

	loginReq, err := buildLoginRequest(reqBody.AuthMethod, reqBody.MethodPayload, login.SignInSelectionExplicit, "method_payload")
	if err != nil {
		h.Error(c, err)
		return
	}
	h.executeLogin(c, loginReq)
}

func buildLoginRequest(method string, payload json.RawMessage, selection login.SignInSelectionMode, payloadLabel string) (login.LoginRequest, error) {
	adapter, ok := loginPayloadAdapters[method]
	if !ok {
		return login.LoginRequest{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication method: %s", method)
	}
	return adapter(payload, selection, payloadLabel)
}

func passwordLoginRequest(payload json.RawMessage, selection login.SignInSelectionMode, payloadLabel string) (login.LoginRequest, error) {
	var creds req.PasswordCredentials
	if err := json.Unmarshal(payload, &creds); err != nil {
		return login.LoginRequest{}, perrors.WithCode(code.ErrBind, "invalid password %s: %v", payloadLabel, err)
	}

	loginReq := login.LoginRequest{
		SelectionMode: selection,
		AuthType:      login.AuthTypePassword,
		Username:      &creds.Username,
		Password:      &creds.Password,
	}
	if creds.TenantID != 0 {
		loginReq.TenantID = meta.FromUint64(creds.TenantID)
	}

	return loginReq, nil
}

func phoneOTPLoginRequest(payload json.RawMessage, selection login.SignInSelectionMode, payloadLabel string) (login.LoginRequest, error) {
	var creds req.PhoneOTPCredentials
	if err := json.Unmarshal(payload, &creds); err != nil {
		return login.LoginRequest{}, perrors.WithCode(code.ErrBind, "invalid phone OTP %s: %v", payloadLabel, err)
	}

	return login.LoginRequest{
		SelectionMode: selection,
		AuthType:      login.AuthTypePhoneOTP,
		PhoneE164:     &creds.Phone,
		OTPCode:       &creds.OTPCode,
	}, nil
}

func wechatLoginRequest(payload json.RawMessage, selection login.SignInSelectionMode, payloadLabel string) (login.LoginRequest, error) {
	var creds req.WeChatCredentials
	if err := json.Unmarshal(payload, &creds); err != nil {
		return login.LoginRequest{}, perrors.WithCode(code.ErrBind, "invalid wechat %s: %v", payloadLabel, err)
	}

	return login.LoginRequest{
		SelectionMode: selection,
		AuthType:      login.AuthTypeWechat,
		WechatAppID:   &creds.AppID,
		WechatJSCode:  &creds.Code,
	}, nil
}

func wecomLoginRequest(payload json.RawMessage, selection login.SignInSelectionMode, payloadLabel string) (login.LoginRequest, error) {
	var creds req.WeComCredentials
	if err := json.Unmarshal(payload, &creds); err != nil {
		return login.LoginRequest{}, perrors.WithCode(code.ErrBind, "invalid wecom %s: %v", payloadLabel, err)
	}

	return login.LoginRequest{
		SelectionMode: selection,
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
