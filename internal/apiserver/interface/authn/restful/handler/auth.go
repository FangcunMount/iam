package handler

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/apiserver/application/authn/login"
	loginprep "github.com/FangcunMount/iam/internal/apiserver/application/authn/loginprep"
	"github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	domainToken "github.com/FangcunMount/iam/internal/apiserver/domain/authn/token"
	req "github.com/FangcunMount/iam/internal/apiserver/interface/authn/restful/request"
	resp "github.com/FangcunMount/iam/internal/apiserver/interface/authn/restful/response"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// AuthHandler 认证 HTTP 处理器
type AuthHandler struct {
	*BaseHandler
	loginService     login.LoginApplicationService
	tokenService     token.TokenApplicationService
	loginPreparation loginprep.LoginPreparationService
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(
	loginService login.LoginApplicationService,
	tokenService token.TokenApplicationService,
	loginPreparation loginprep.LoginPreparationService,
) *AuthHandler {
	return &AuthHandler{
		BaseHandler:      NewBaseHandler(),
		loginService:     loginService,
		tokenService:     tokenService,
		loginPreparation: loginPreparation,
	}
}

// Login 统一登录端点
// @Summary 用户登录
// @Description 支持多种登录方式：密码登录、手机验证码登录、微信小程序登录、企业微信登录
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

	// 根据认证方法路由到对应的处理函数
	switch reqBody.Method {
	case "password":
		h.handlePasswordLogin(c, reqBody)
	case "phone_otp":
		h.handlePhoneOTPLogin(c, reqBody)
	case "wechat":
		h.handleWeChatLogin(c, reqBody)
	case "wecom":
		h.handleWeComLogin(c, reqBody)
	default:
		h.Error(c, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication method: %s", reqBody.Method))
	}
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

	switch reqBody.AuthMethod {
	case "password":
		h.handlePasswordLoginV2(c, reqBody)
	case "phone_otp":
		h.handlePhoneOTPLoginV2(c, reqBody)
	case "wechat":
		h.handleWeChatLoginV2(c, reqBody)
	case "wecom":
		h.handleWeComLoginV2(c, reqBody)
	default:
		h.Error(c, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication method: %s", reqBody.AuthMethod))
	}
}

// handlePasswordLogin 处理密码登录
func (h *AuthHandler) handlePasswordLogin(c *gin.Context, reqBody req.LoginRequest) {
	var creds req.PasswordCredentials
	if err := json.Unmarshal(reqBody.Credentials, &creds); err != nil {
		h.Error(c, perrors.WithCode(code.ErrBind, "invalid password credentials: %v", err))
		return
	}

	loginReq := login.LoginRequest{
		AuthType: login.AuthTypePassword,
		Username: &creds.Username,
		Password: &creds.Password,
	}
	if creds.TenantID != 0 {
		loginReq.TenantID = meta.FromUint64(creds.TenantID)
	}

	h.executeLogin(c, loginReq)
}

func (h *AuthHandler) handlePasswordLoginV2(c *gin.Context, reqBody req.LoginV2Request) {
	var creds req.PasswordCredentials
	if err := json.Unmarshal(reqBody.MethodPayload, &creds); err != nil {
		h.Error(c, perrors.WithCode(code.ErrBind, "invalid password method_payload: %v", err))
		return
	}

	loginReq := login.LoginRequest{
		SelectionMode: login.ScenarioSelectionExplicit,
		AuthType:      login.AuthTypePassword,
		Username:      &creds.Username,
		Password:      &creds.Password,
	}
	if creds.TenantID != 0 {
		loginReq.TenantID = meta.FromUint64(creds.TenantID)
	}

	h.executeLogin(c, loginReq)
}

// handlePhoneOTPLogin 处理手机验证码登录
func (h *AuthHandler) handlePhoneOTPLogin(c *gin.Context, reqBody req.LoginRequest) {
	var creds req.PhoneOTPCredentials
	if err := json.Unmarshal(reqBody.Credentials, &creds); err != nil {
		h.Error(c, perrors.WithCode(code.ErrBind, "invalid phone OTP credentials: %v", err))
		return
	}

	loginReq := login.LoginRequest{
		AuthType:  login.AuthTypePhoneOTP,
		PhoneE164: &creds.Phone,
		OTPCode:   &creds.OTPCode,
	}

	h.executeLogin(c, loginReq)
}

func (h *AuthHandler) handlePhoneOTPLoginV2(c *gin.Context, reqBody req.LoginV2Request) {
	var creds req.PhoneOTPCredentials
	if err := json.Unmarshal(reqBody.MethodPayload, &creds); err != nil {
		h.Error(c, perrors.WithCode(code.ErrBind, "invalid phone OTP method_payload: %v", err))
		return
	}

	loginReq := login.LoginRequest{
		SelectionMode: login.ScenarioSelectionExplicit,
		AuthType:      login.AuthTypePhoneOTP,
		PhoneE164:     &creds.Phone,
		OTPCode:       &creds.OTPCode,
	}

	h.executeLogin(c, loginReq)
}

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

// handleWeChatLogin 处理微信小程序登录
func (h *AuthHandler) handleWeChatLogin(c *gin.Context, reqBody req.LoginRequest) {
	var creds req.WeChatCredentials
	if err := json.Unmarshal(reqBody.Credentials, &creds); err != nil {
		h.Error(c, perrors.WithCode(code.ErrBind, "invalid wechat credentials: %v", err))
		return
	}

	loginReq := login.LoginRequest{
		AuthType:     login.AuthTypeWechat,
		WechatAppID:  &creds.AppID,
		WechatJSCode: &creds.Code,
	}

	h.executeLogin(c, loginReq)
}

func (h *AuthHandler) handleWeChatLoginV2(c *gin.Context, reqBody req.LoginV2Request) {
	var creds req.WeChatCredentials
	if err := json.Unmarshal(reqBody.MethodPayload, &creds); err != nil {
		h.Error(c, perrors.WithCode(code.ErrBind, "invalid wechat method_payload: %v", err))
		return
	}

	loginReq := login.LoginRequest{
		SelectionMode: login.ScenarioSelectionExplicit,
		AuthType:      login.AuthTypeWechat,
		WechatAppID:   &creds.AppID,
		WechatJSCode:  &creds.Code,
	}

	h.executeLogin(c, loginReq)
}

// handleWeComLogin 处理企业微信登录
func (h *AuthHandler) handleWeComLogin(c *gin.Context, reqBody req.LoginRequest) {
	var creds req.WeComCredentials
	if err := json.Unmarshal(reqBody.Credentials, &creds); err != nil {
		h.Error(c, perrors.WithCode(code.ErrBind, "invalid wecom credentials: %v", err))
		return
	}

	loginReq := login.LoginRequest{
		AuthType:    login.AuthTypeWecom,
		WecomCorpID: &creds.CorpID,
		WecomCode:   &creds.AuthCode,
	}

	h.executeLogin(c, loginReq)
}

func (h *AuthHandler) handleWeComLoginV2(c *gin.Context, reqBody req.LoginV2Request) {
	var creds req.WeComCredentials
	if err := json.Unmarshal(reqBody.MethodPayload, &creds); err != nil {
		h.Error(c, perrors.WithCode(code.ErrBind, "invalid wecom method_payload: %v", err))
		return
	}

	loginReq := login.LoginRequest{
		SelectionMode: login.ScenarioSelectionExplicit,
		AuthType:      login.AuthTypeWecom,
		WecomCorpID:   &creds.CorpID,
		WecomCode:     &creds.AuthCode,
	}

	h.executeLogin(c, loginReq)
}

// executeLogin 执行登录并返回令牌
func (h *AuthHandler) executeLogin(c *gin.Context, loginReq login.LoginRequest) {
	result, err := h.loginService.Login(c.Request.Context(), loginReq)
	if err != nil {
		h.Error(c, err)
		return
	}

	// 转换为 HTTP 响应格式
	tokenPair := h.convertTokenPair(result.TokenPair)
	h.Success(c, tokenPair)
}

// Logout 登出
// @Summary 用户登出
// @Description 撤销访问令牌和刷新令牌
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body req.LogoutRequest true "登出请求"
// @Success 200 {object} resp.MessageResponse "登出成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Router /authn/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	var reqBody req.LogoutRequest
	if err := h.BindJSON(c, &reqBody); err != nil {
		h.Error(c, err)
		return
	}

	if err := reqBody.Validate(); err != nil {
		h.Error(c, err)
		return
	}

	logoutReq := login.LogoutRequest{
		AccessToken:  reqBody.AccessToken,
		RefreshToken: &reqBody.RefreshToken,
	}

	if err := h.loginService.Logout(c.Request.Context(), logoutReq); err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, resp.MessageResponse{Message: "Logout successful"})
}

// RefreshToken 刷新访问令牌
// @Summary 刷新访问令牌
// @Description 使用刷新令牌获取新的访问令牌
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body req.RefreshTokenRequest true "刷新令牌请求"
// @Success 200 {object} resp.TokenPair "刷新成功，返回新的访问令牌"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 401 {object} map[string]interface{} "刷新令牌无效或已过期"
// @Router /authn/refresh_token [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var reqBody req.RefreshTokenRequest
	if err := h.BindJSON(c, &reqBody); err != nil {
		h.Error(c, err)
		return
	}

	if err := reqBody.Validate(); err != nil {
		h.Error(c, err)
		return
	}

	result, err := h.tokenService.RefreshToken(c.Request.Context(), reqBody.RefreshToken)
	if err != nil {
		h.Error(c, err)
		return
	}

	tokenPair := h.convertTokenPair(result.TokenPair)
	h.Success(c, tokenPair)
}

// VerifyToken 验证访问令牌
// @Summary 验证访问令牌
// @Description 验证访问令牌的有效性并返回声明信息
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body req.VerifyTokenRequest true "验证令牌请求"
// @Success 200 {object} resp.TokenVerifyResponse "验证成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 401 {object} map[string]interface{} "令牌无效"
// @Router /authn/verify [post]
func (h *AuthHandler) VerifyToken(c *gin.Context) {
	var reqBody req.VerifyTokenRequest
	if err := h.BindJSON(c, &reqBody); err != nil {
		h.Error(c, err)
		return
	}

	if err := reqBody.Validate(); err != nil {
		h.Error(c, err)
		return
	}

	result, err := h.tokenService.VerifyToken(c.Request.Context(), token.VerifyTokenRequest{
		AccessToken:      reqBody.AccessToken,
		ExpectedIssuer:   strings.TrimSpace(reqBody.ExpectedIssuer),
		ExpectedAudience: append([]string(nil), reqBody.ExpectedAudience...),
	})
	if err != nil {
		h.Error(c, err)
		return
	}

	response := resp.TokenVerifyResponse{
		Valid:  result.Valid,
		Claims: nil,
	}

	if result.Valid && result.Claims != nil {
		var tenantID *int64
		if !result.Claims.TenantID.IsZero() {
			value := int64(result.Claims.TenantID.Uint64())
			tenantID = &value
		}
		amr := append([]string(nil), result.Claims.AMR...)
		attrs := result.Claims.Attributes
		var attrCopy map[string]string
		if len(attrs) > 0 {
			attrCopy = make(map[string]string, len(attrs))
			for k, v := range attrs {
				attrCopy[k] = v
			}
		}
		response.Claims = &resp.TokenClaims{
			UserID:     result.Claims.UserID.String(),
			AccountID:  result.Claims.AccountID.String(),
			TenantID:   tenantID,
			Issuer:     result.Claims.Issuer,
			IssuedAt:   result.Claims.IssuedAt,
			ExpiresAt:  result.Claims.ExpiresAt,
			JTI:        result.Claims.TokenID,
			Amr:        amr,
			Attributes: attrCopy,
		}
	}

	h.Success(c, response)
}

// RevokeToken 撤销访问令牌
func (h *AuthHandler) RevokeToken(c *gin.Context) {
	var reqBody req.RevokeTokenRequest
	if err := h.BindJSON(c, &reqBody); err != nil {
		h.Error(c, err)
		return
	}

	if err := reqBody.Validate(); err != nil {
		h.Error(c, err)
		return
	}

	if err := h.tokenService.RevokeAccessToken(c.Request.Context(), reqBody.AccessToken); err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, resp.MessageResponse{Message: "Token revoked successfully"})
}

// RevokeRefreshToken 撤销刷新令牌
func (h *AuthHandler) RevokeRefreshToken(c *gin.Context) {
	var reqBody req.RevokeRefreshTokenRequest
	if err := h.BindJSON(c, &reqBody); err != nil {
		h.Error(c, err)
		return
	}

	if err := reqBody.Validate(); err != nil {
		h.Error(c, err)
		return
	}

	if err := h.tokenService.RevokeRefreshToken(c.Request.Context(), reqBody.RefreshToken); err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, resp.MessageResponse{Message: "Refresh token revoked successfully"})
}

// convertTokenPair 转换令牌对为 HTTP 响应格式
func (h *AuthHandler) convertTokenPair(tokenPair *domainToken.TokenPair) *resp.TokenPair {
	response := &resp.TokenPair{
		TokenType: "Bearer",
	}

	if tokenPair == nil {
		return response
	}

	if tokenPair.AccessToken != nil {
		response.AccessToken = tokenPair.AccessToken.Value
		response.ExpiresIn = int64(time.Until(tokenPair.AccessToken.ExpiresAt).Seconds())
	}

	if tokenPair.RefreshToken != nil {
		response.RefreshToken = tokenPair.RefreshToken.Value
	}

	return response
}
