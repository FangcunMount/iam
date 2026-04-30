package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	jwksApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/jwks"
	"github.com/FangcunMount/iam/pkg/core"
)

var _ = core.ErrResponse{}

// GetJWKS 获取 JWKS（公开端点）
// @Summary 获取 JWKS
// @Description 获取 JSON Web Key Set，用于验证 JWT 签名
// @Tags Authentication-JWKS
// @Produce json
// @Success 200 {object} map[string]interface{} "JWKS JSON"
// @Header 200 {string} ETag "实体标签"
// @Header 200 {string} Last-Modified "最后修改时间"
// @Header 200 {string} Cache-Control "缓存控制"
// @Failure 500 {object} core.ErrResponse "服务器错误"
// @Router /.well-known/jwks.json [get]
func (h *JWKSHandler) GetJWKS(c *gin.Context) {
	ctx := c.Request.Context()

	result, err := h.keyPublishApp.BuildJWKS(ctx)
	if err != nil {
		h.Error(c, err)
		return
	}

	if clientJWKSCacheMatches(c, result) {
		c.AbortWithStatus(http.StatusNotModified)
		return
	}

	writeJWKSCacheHeaders(c, result)
	c.Data(http.StatusOK, "application/json", result.JWKS)
}

func clientJWKSCacheMatches(c *gin.Context, result *jwksApp.BuildJWKSResponse) bool {
	clientETag := c.GetHeader("If-None-Match")
	return clientETag != "" && clientETag == result.ETag
}

func writeJWKSCacheHeaders(c *gin.Context, result *jwksApp.BuildJWKSResponse) {
	c.Header("Content-Type", "application/json")
	c.Header("ETag", result.ETag)
	c.Header("Last-Modified", result.LastModified.Format(http.TimeFormat))
	c.Header("Cache-Control", "public, max-age=3600")
}
