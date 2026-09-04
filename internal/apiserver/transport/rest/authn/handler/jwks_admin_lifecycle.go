package handler

import (
	"github.com/gin-gonic/gin"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	jwksApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/jwks"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authn/response"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/pkg/core"
)

var _ = core.ErrResponse{}

// RetireKey 退役密钥（管理员接口）
// @Summary 退役密钥
// @Description 将密钥状态从 Grace 转为 Retired
// @Tags Authentication-JWKS
// @Produce json
// @Security BearerAuth
// @Param kid path string true "密钥 ID"
// @Success 204 "退役成功"
// @Failure 400 {object} core.ErrResponse "参数错误或状态不允许"
// @Failure 401 {object} core.ErrResponse "未认证"
// @Failure 403 {object} core.ErrResponse "无权限"
// @Failure 404 {object} core.ErrResponse "密钥不存在"
// @Failure 500 {object} core.ErrResponse "服务器错误"
// @Router /v2/authn/admin/jwks/keys/{kid}/retire [post]
func (h *JWKSHandler) RetireKey(c *gin.Context) {
	ctx := c.Request.Context()
	kid := c.Param("kid")

	if kid == "" {
		h.Error(c, perrors.WithCode(code.ErrInvalidArgument, "kid is required"))
		return
	}

	if err := h.keyLifecycleApp.RetireKey(ctx, kid); err != nil {
		h.Error(c, err)
		return
	}

	h.NoContent(c)
}

// ForceRetireKey 强制退役密钥（管理员接口）
// @Summary 强制退役密钥
// @Description 强制将任何状态的密钥转为 Retired（用于紧急情况）
// @Tags Authentication-JWKS
// @Produce json
// @Security BearerAuth
// @Param kid path string true "密钥 ID"
// @Success 204 "强制退役成功"
// @Failure 400 {object} core.ErrResponse "参数错误"
// @Failure 401 {object} core.ErrResponse "未认证"
// @Failure 403 {object} core.ErrResponse "无权限"
// @Failure 404 {object} core.ErrResponse "密钥不存在"
// @Failure 500 {object} core.ErrResponse "服务器错误"
// @Router /v2/authn/admin/jwks/keys/{kid}/force-retire [post]
func (h *JWKSHandler) ForceRetireKey(c *gin.Context) {
	ctx := c.Request.Context()
	kid := c.Param("kid")

	if kid == "" {
		h.Error(c, perrors.WithCode(code.ErrInvalidArgument, "kid is required"))
		return
	}

	if err := h.keyLifecycleApp.ForceRetireKey(ctx, kid); err != nil {
		h.Error(c, err)
		return
	}

	h.NoContent(c)
}

// CleanupExpiredKeys 清理过期密钥（管理员接口）
// @Summary 清理过期密钥
// @Description 删除已过期的 Retired 密钥
// @Tags Authentication-JWKS
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.CleanupResponse "清理结果"
// @Failure 401 {object} core.ErrResponse "未认证"
// @Failure 403 {object} core.ErrResponse "无权限"
// @Failure 500 {object} core.ErrResponse "服务器错误"
// @Router /v2/authn/admin/jwks/keys/cleanup [post]
func (h *JWKSHandler) CleanupExpiredKeys(c *gin.Context) {
	ctx := c.Request.Context()

	result, err := h.keyLifecycleApp.CleanupExpiredKeys(ctx)
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, &response.CleanupResponse{DeletedCount: result.DeletedCount})
}

// GetPublishableKeys 获取可发布的密钥（管理员接口）
// @Summary 获取可发布的密钥
// @Description 获取当前会被发布到 JWKS 的密钥列表（用于预览或调试）
// @Tags Authentication-JWKS
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.PublishableKeysResponse "可发布的密钥列表"
// @Failure 401 {object} core.ErrResponse "未认证"
// @Failure 403 {object} core.ErrResponse "无权限"
// @Failure 500 {object} core.ErrResponse "服务器错误"
// @Router /v2/authn/admin/jwks/keys/publishable [get]
func (h *JWKSHandler) GetPublishableKeys(c *gin.Context) {
	ctx := c.Request.Context()

	result, err := h.keyPublishApp.GetPublishableKeys(ctx)
	if err != nil {
		h.Error(c, err)
		return
	}

	keys := make([]*response.PublishableKeyInfo, len(result.Keys))
	for i, key := range result.Keys {
		keys[i] = &response.PublishableKeyInfo{
			Kid:       key.Kid,
			Status:    key.Status,
			Algorithm: key.Algorithm,
			NotBefore: key.NotBefore,
			NotAfter:  key.NotAfter,
			PublicJWK: restPublicJWKFromPublished(key.PublicJWK),
		}
	}

	h.Success(c, &response.PublishableKeysResponse{Keys: keys})
}

func restPublicJWKFromPublished(jwk *jwksApp.PublicJWK) *response.PublicJWK {
	if jwk == nil {
		return nil
	}
	return &response.PublicJWK{
		Kty: jwk.Kty, Use: jwk.Use, Alg: jwk.Alg, Kid: jwk.Kid,
		N: jwk.N, E: jwk.E, Crv: jwk.Crv, X: jwk.X, Y: jwk.Y,
	}
}
