package handler

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/session"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/token"
	req "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authn/request"
	resp "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authn/response"
)

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

	logoutReq := session.LogoutRequest{
		AccessToken:  reqBody.AccessToken,
		RefreshToken: &reqBody.RefreshToken,
	}

	if err := h.sessionService.Logout(c.Request.Context(), logoutReq); err != nil {
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

	result, err := h.sessionService.RenewSession(c.Request.Context(), reqBody.RefreshToken)
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

	result, err := h.tokenVerifier.VerifyToken(c.Request.Context(), token.VerifyTokenRequest{
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
			UserID:          result.Claims.UserID.String(),
			LoginIdentityID: result.Claims.LoginIdentityID.String(),
			TenantID:        tenantID,
			Issuer:          result.Claims.Issuer,
			IssuedAt:        result.Claims.IssuedAt,
			ExpiresAt:       result.Claims.ExpiresAt,
			JTI:             result.Claims.TokenID,
			Amr:             amr,
			Attributes:      attrCopy,
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

	if err := h.tokenRevoker.RevokeAccessToken(c.Request.Context(), reqBody.AccessToken); err != nil {
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

	if err := h.tokenRevoker.RevokeRefreshToken(c.Request.Context(), reqBody.RefreshToken); err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, resp.MessageResponse{Message: "Refresh token revoked successfully"})
}

// convertTokenPair 转换令牌对为 HTTP 响应格式
func (h *AuthHandler) convertTokenPair(tokenPair *token.TokenPair) *resp.TokenPair {
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
