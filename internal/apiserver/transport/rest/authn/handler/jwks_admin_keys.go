package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	jwksApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/jwks"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authn/request"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authn/response"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/pkg/core"
)

var _ = core.ErrResponse{}

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

	appReq := jwksApp.CreateKeyRequest{
		Algorithm: req.Algorithm,
		NotBefore: req.NotBefore,
		NotAfter:  req.NotAfter,
	}

	result, err := h.keyLifecycleApp.CreateAndActivate(ctx, appReq)
	if err != nil {
		h.Error(c, err)
		return
	}

	c.JSON(http.StatusCreated, createKeyResponseFromResult(result))
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

	statusStr := c.DefaultQuery("status", "")
	limit := c.DefaultQuery("limit", "20")
	offset := c.DefaultQuery("offset", "0")

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

	var status string
	if statusStr != "" {
		status, err = parseKeyStatus(statusStr)
		if err != nil {
			h.Error(c, err)
			return
		}
	}

	result, err := h.keyManagementApp.ListKeys(ctx, jwksApp.ListKeysRequest{
		Status: status,
		Limit:  limitInt,
		Offset: offsetInt,
	})
	if err != nil {
		h.Error(c, err)
		return
	}

	keys := make([]*response.KeyInfo, len(result.Keys))
	for i, key := range result.Keys {
		keys[i] = &response.KeyInfo{
			Kid:       key.Kid,
			Status:    key.Status,
			Algorithm: key.Algorithm,
			NotBefore: key.NotBefore,
			NotAfter:  key.NotAfter,
			PublicJWK: key.PublicJWK,
			CreatedAt: key.CreatedAt,
			UpdatedAt: key.UpdatedAt,
		}
	}

	h.Success(c, &response.KeyListResponse{
		Keys:   keys,
		Total:  result.Total,
		Limit:  limitInt,
		Offset: offsetInt,
	})
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

	h.Success(c, keyByKidResponseFromResult(result))
}

func createKeyResponseFromResult(result *jwksApp.CreateKeyResponse) *response.KeyResponse {
	if result == nil {
		return nil
	}
	return &response.KeyResponse{
		Kid:       result.Kid,
		Status:    result.Status,
		Algorithm: result.Algorithm,
		NotBefore: result.NotBefore,
		NotAfter:  result.NotAfter,
		PublicJWK: result.PublicJWK,
		CreatedAt: result.CreatedAt,
	}
}

func keyByKidResponseFromResult(result *jwksApp.GetKeyByKidResponse) *response.KeyResponse {
	if result == nil {
		return nil
	}
	return &response.KeyResponse{
		Kid:       result.Kid,
		Status:    result.Status,
		Algorithm: result.Algorithm,
		NotBefore: result.NotBefore,
		NotAfter:  result.NotAfter,
		PublicJWK: result.PublicJWK,
		CreatedAt: result.CreatedAt,
		UpdatedAt: result.UpdatedAt,
	}
}
