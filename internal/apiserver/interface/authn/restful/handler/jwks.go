package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	jwksApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/jwks"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/jwks"
	"github.com/FangcunMount/iam/internal/apiserver/interface/authn/restful/request"
	"github.com/FangcunMount/iam/internal/apiserver/interface/authn/restful/response"
	"github.com/FangcunMount/iam/internal/pkg/code"
	_ "github.com/FangcunMount/iam/pkg/core" // imported for swagger
)

// JWKSHandler JWKS HTTP 处理器
type JWKSHandler struct {
	*BaseHandler
	keyManagementApp *jwksApp.KeyManagementAppService
	keyPublishApp    *jwksApp.KeyPublishAppService
}

// NewJWKSHandler 创建 JWKS 处理器
func NewJWKSHandler(
	keyManagementApp *jwksApp.KeyManagementAppService,
	keyPublishApp *jwksApp.KeyPublishAppService,
) *JWKSHandler {
	return &JWKSHandler{
		BaseHandler:      NewBaseHandler(),
		keyManagementApp: keyManagementApp,
		keyPublishApp:    keyPublishApp,
	}
}

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

	// 构建 JWKS
	result, err := h.keyPublishApp.BuildJWKS(ctx)
	if err != nil {
		h.Error(c, err)
		return
	}

	// 检查客户端缓存
	clientETag := c.GetHeader("If-None-Match")
	if clientETag != "" && clientETag == result.ETag {
		c.AbortWithStatus(http.StatusNotModified)
		return
	}

	// 设置缓存头
	c.Header("Content-Type", "application/json")
	c.Header("ETag", result.ETag)
	c.Header("Last-Modified", result.LastModified.Format(http.TimeFormat))
	c.Header("Cache-Control", "public, max-age=3600") // 缓存 1 小时

	// 返回 JWKS JSON（直接写入原始 JSON）
	c.Data(http.StatusOK, "application/json", result.JWKS)
}

// CreateKey 创建密钥（管理员接口）
// @Summary 创建密钥
// @Description 创建新的签名密钥
// @Tags Authentication-JWKS
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body request.CreateKeyRequest true "创建密钥请求"
// @Success 201 {object} response.KeyResponse "创建成功"
// @Failure 400 {object} core.ErrResponse "参数错误"
// @Failure 401 {object} core.ErrResponse "未认证"
// @Failure 403 {object} core.ErrResponse "无权限"
// @Failure 500 {object} core.ErrResponse "服务器错误"
// @Router /authn/admin/jwks/keys [post]
func (h *JWKSHandler) CreateKey(c *gin.Context) {
	ctx := c.Request.Context()

	var req request.CreateKeyRequest
	if err := h.BindJSON(c, &req); err != nil {
		h.Error(c, err)
		return
	}

	// 调用应用服务
	appReq := jwksApp.CreateKeyRequest{
		Algorithm: req.Algorithm,
		NotBefore: req.NotBefore,
		NotAfter:  req.NotAfter,
	}

	result, err := h.keyManagementApp.CreateKey(ctx, appReq)
	if err != nil {
		h.Error(c, err)
		return
	}

	// 转换为响应 DTO
	resp := &response.KeyResponse{
		Kid:       result.Kid,
		Status:    result.Status.String(),
		Algorithm: result.Algorithm,
		NotBefore: result.NotBefore,
		NotAfter:  result.NotAfter,
		PublicJWK: result.PublicJWK,
		CreatedAt: result.CreatedAt,
	}

	c.JSON(http.StatusCreated, resp)
}

// ListKeys 列出密钥（管理员接口）
// @Summary 列出密钥
// @Description 分页列出所有密钥
// @Tags Authentication-JWKS
// @Produce json
// @Security BearerAuth
// @Param status query string false "状态过滤 (active, grace, retired)"
// @Param limit query int false "每页数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} response.KeyListResponse "密钥列表"
// @Failure 400 {object} core.ErrResponse "参数错误"
// @Failure 401 {object} core.ErrResponse "未认证"
// @Failure 403 {object} core.ErrResponse "无权限"
// @Failure 500 {object} core.ErrResponse "服务器错误"
// @Router /authn/admin/jwks/keys [get]
func (h *JWKSHandler) ListKeys(c *gin.Context) {
	ctx := c.Request.Context()

	// 解析查询参数
	statusStr := c.DefaultQuery("status", "")
	limit := c.DefaultQuery("limit", "20")
	offset := c.DefaultQuery("offset", "0")

	// 转换参数
	limitInt, err := parsePositiveInt(limit, "limit")
	if err != nil {
		h.Error(c, err)
		return
	}

	offsetInt, err := parseNonNegativeInt(offset, "offset")
	if err != nil {
		h.Error(c, err)
		return
	}

	// 解析状态
	var status jwks.KeyStatus
	if statusStr != "" {
		statusUint, err := parseKeyStatus(statusStr)
		if err != nil {
			h.Error(c, err)
			return
		}
		status = jwks.KeyStatus(statusUint)
	}

	// 调用应用服务
	appReq := jwksApp.ListKeysRequest{
		Status: status,
		Limit:  limitInt,
		Offset: offsetInt,
	}

	result, err := h.keyManagementApp.ListKeys(ctx, appReq)
	if err != nil {
		h.Error(c, err)
		return
	}

	// 转换为响应 DTO
	keys := make([]*response.KeyInfo, len(result.Keys))
	for i, key := range result.Keys {
		keys[i] = &response.KeyInfo{
			Kid:       key.Kid,
			Status:    key.Status.String(),
			Algorithm: key.Algorithm,
			NotBefore: key.NotBefore,
			NotAfter:  key.NotAfter,
			PublicJWK: key.PublicJWK,
			CreatedAt: key.CreatedAt,
			UpdatedAt: key.UpdatedAt,
		}
	}

	resp := &response.KeyListResponse{
		Keys:   keys,
		Total:  result.Total,
		Limit:  limitInt,
		Offset: offsetInt,
	}

	h.Success(c, resp)
}

// GetKey 获取密钥详情（管理员接口）
// @Summary 获取密钥详情
// @Description 根据 kid 获取密钥详细信息
// @Tags Authentication-JWKS
// @Produce json
// @Security BearerAuth
// @Param kid path string true "密钥 ID"
// @Success 200 {object} response.KeyResponse "密钥详情"
// @Failure 400 {object} core.ErrResponse "参数错误"
// @Failure 401 {object} core.ErrResponse "未认证"
// @Failure 403 {object} core.ErrResponse "无权限"
// @Failure 404 {object} core.ErrResponse "密钥不存在"
// @Failure 500 {object} core.ErrResponse "服务器错误"
// @Router /authn/admin/jwks/keys/{kid} [get]
func (h *JWKSHandler) GetKey(c *gin.Context) {
	ctx := c.Request.Context()
	kid := c.Param("kid")

	if kid == "" {
		h.Error(c, perrors.WithCode(code.ErrInvalidArgument, "kid is required"))
		return
	}

	result, err := h.keyManagementApp.GetKeyByKid(ctx, kid)
	if err != nil {
		h.Error(c, err)
		return
	}

	resp := &response.KeyResponse{
		Kid:       result.Kid,
		Status:    result.Status.String(),
		Algorithm: result.Algorithm,
		NotBefore: result.NotBefore,
		NotAfter:  result.NotAfter,
		PublicJWK: result.PublicJWK,
		CreatedAt: result.CreatedAt,
		UpdatedAt: result.UpdatedAt,
	}

	h.Success(c, resp)
}

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
// @Router /authn/admin/jwks/keys/{kid}/retire [post]
func (h *JWKSHandler) RetireKey(c *gin.Context) {
	ctx := c.Request.Context()
	kid := c.Param("kid")

	if kid == "" {
		h.Error(c, perrors.WithCode(code.ErrInvalidArgument, "kid is required"))
		return
	}

	if err := h.keyManagementApp.RetireKey(ctx, kid); err != nil {
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
// @Router /authn/admin/jwks/keys/{kid}/force-retire [post]
func (h *JWKSHandler) ForceRetireKey(c *gin.Context) {
	ctx := c.Request.Context()
	kid := c.Param("kid")

	if kid == "" {
		h.Error(c, perrors.WithCode(code.ErrInvalidArgument, "kid is required"))
		return
	}

	if err := h.keyManagementApp.ForceRetireKey(ctx, kid); err != nil {
		h.Error(c, err)
		return
	}

	h.NoContent(c)
}

// EnterGracePeriod 进入宽限期（管理员接口）
// @Summary 进入宽限期
// @Description 将密钥状态从 Active 转为 Grace
// @Tags Authentication-JWKS
// @Produce json
// @Security BearerAuth
// @Param kid path string true "密钥 ID"
// @Success 204 "进入宽限期成功"
// @Failure 400 {object} core.ErrResponse "参数错误或状态不允许"
// @Failure 401 {object} core.ErrResponse "未认证"
// @Failure 403 {object} core.ErrResponse "无权限"
// @Failure 404 {object} core.ErrResponse "密钥不存在"
// @Failure 500 {object} core.ErrResponse "服务器错误"
// @Router /authn/admin/jwks/keys/{kid}/grace [post]
func (h *JWKSHandler) EnterGracePeriod(c *gin.Context) {
	ctx := c.Request.Context()
	kid := c.Param("kid")

	if kid == "" {
		h.Error(c, perrors.WithCode(code.ErrInvalidArgument, "kid is required"))
		return
	}

	if err := h.keyManagementApp.EnterGracePeriod(ctx, kid); err != nil {
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
// @Router /authn/admin/jwks/keys/cleanup [post]
func (h *JWKSHandler) CleanupExpiredKeys(c *gin.Context) {
	ctx := c.Request.Context()

	result, err := h.keyManagementApp.CleanupExpiredKeys(ctx)
	if err != nil {
		h.Error(c, err)
		return
	}

	resp := &response.CleanupResponse{
		DeletedCount: result.DeletedCount,
	}

	h.Success(c, resp)
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
// @Router /authn/admin/jwks/keys/publishable [get]
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
			Status:    key.Status.String(),
			Algorithm: key.Algorithm,
			NotBefore: key.NotBefore,
			NotAfter:  key.NotAfter,
			PublicJWK: key.PublicJWK,
		}
	}

	resp := &response.PublishableKeysResponse{
		Keys: keys,
	}

	h.Success(c, resp)
}

// ==================== 辅助函数 ====================

// parsePositiveInt 解析正整数
func parsePositiveInt(value, field string) (int, error) {
	var result int
	if _, err := fmt.Sscanf(value, "%d", &result); err != nil {
		return 0, perrors.WithCode(code.ErrInvalidArgument, "invalid %s: %s", field, value)
	}
	if result <= 0 {
		return 0, perrors.WithCode(code.ErrInvalidArgument, "%s must be positive", field)
	}
	return result, nil
}

// parseNonNegativeInt 解析非负整数
func parseNonNegativeInt(value, field string) (int, error) {
	var result int
	if _, err := fmt.Sscanf(value, "%d", &result); err != nil {
		return 0, perrors.WithCode(code.ErrInvalidArgument, "invalid %s: %s", field, value)
	}
	if result < 0 {
		return 0, perrors.WithCode(code.ErrInvalidArgument, "%s cannot be negative", field)
	}
	return result, nil
}

// parseKeyStatus 解析密钥状态字符串
func parseKeyStatus(status string) (uint8, error) {
	switch strings.ToLower(status) {
	case "active":
		return 1, nil
	case "grace":
		return 2, nil
	case "retired":
		return 3, nil
	default:
		return 0, perrors.WithCode(code.ErrInvalidArgument, "invalid status: %s (must be active, grace, or retired)", status)
	}
}
