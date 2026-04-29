// Package handler 资源管理处理器
package handler

import (
	"strconv"

	"github.com/FangcunMount/component-base/pkg/errors"
	resourceDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/internal/apiserver/transport/rest/authz/dto"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/gin-gonic/gin"
)

// ResourceHandler 资源处理器
//
// 依赖倒置原则：Handler 依赖 driving 接口，不依赖具体实现
type ResourceHandler struct {
	commander resourceDomain.Commander // 命令服务（写操作）
	queryer   resourceDomain.Queryer   // 查询服务（读操作）
}

// NewResourceHandler 创建资源处理器
func NewResourceHandler(
	commander resourceDomain.Commander,
	queryer resourceDomain.Queryer,
) *ResourceHandler {
	return &ResourceHandler{
		commander: commander,
		queryer:   queryer,
	}
}

// CreateResource 创建资源
// @Summary 创建资源
// @Tags Authorization-Resources
// @Accept json
// @Produce json
// @Param request body dto.CreateResourceRequest true "创建资源请求"
// @Success 200 {object} dto.Response{data=dto.ResourceResponse}
// @Router /authz/resources [post]
func (h *ResourceHandler) CreateResource(c *gin.Context) {
	var req dto.CreateResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleError(c, errors.WithCode(code.ErrBind, "请求参数错误: %v", err))
		return
	}

	cmd := resourceDomain.CreateResourceCommand{
		Key:         req.Key,
		DisplayName: req.DisplayName,
		AppName:     req.AppName,
		Domain:      req.Domain,
		Type:        req.Type,
		Actions:     req.Actions,
		Description: req.Description,
	}

	createdResource, err := h.commander.CreateResource(c.Request.Context(), cmd)
	if err != nil {
		handleError(c, err)
		return
	}

	success(c, h.toResourceResponse(createdResource))
}

// UpdateResource 更新资源
// @Summary 更新资源
// @Tags Authorization-Resources
// @Accept json
// @Produce json
// @Param id path string true "资源ID"
// @Param request body dto.UpdateResourceRequest true "更新资源请求"
// @Success 200 {object} dto.Response{data=dto.ResourceResponse}
// @Router /authz/resources/{id} [put]
func (h *ResourceHandler) UpdateResource(c *gin.Context) {
	resourceID, err := meta.ParseID(c.Param("id"))
	if err != nil {
		handleError(c, errors.WithCode(code.ErrInvalidArgument, "资源ID格式错误"))
		return
	}

	var req dto.UpdateResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleError(c, errors.WithCode(code.ErrBind, "请求参数错误: %v", err))
		return
	}

	cmd := resourceDomain.UpdateResourceCommand{
		ID:          resourceDomain.NewResourceID(resourceID.Uint64()),
		DisplayName: &req.DisplayName,
		Actions:     req.Actions,
		Description: &req.Description,
	}

	updatedResource, err := h.commander.UpdateResource(c.Request.Context(), cmd)
	if err != nil {
		handleError(c, err)
		return
	}

	success(c, h.toResourceResponse(updatedResource))
}

// DeleteResource 删除资源
// @Summary 删除资源
// @Tags Authorization-Resources
// @Param id path string true "资源ID"
// @Success 200 {object} dto.Response
// @Router /authz/resources/{id} [delete]
func (h *ResourceHandler) DeleteResource(c *gin.Context) {
	resourceID, err := meta.ParseID(c.Param("id"))
	if err != nil {
		handleError(c, errors.WithCode(code.ErrInvalidArgument, "资源ID格式错误"))
		return
	}

	err = h.commander.DeleteResource(c.Request.Context(), resourceDomain.NewResourceID(resourceID.Uint64()))
	if err != nil {
		handleError(c, err)
		return
	}

	successNoContent(c)
}

// GetResource 获取资源详情
// @Summary 获取资源详情
// @Tags Authorization-Resources
// @Produce json
// @Param id path string true "资源ID"
// @Success 200 {object} dto.Response{data=dto.ResourceResponse}
// @Router /authz/resources/{id} [get]
func (h *ResourceHandler) GetResource(c *gin.Context) {
	resourceID, err := meta.ParseID(c.Param("id"))
	if err != nil {
		handleError(c, errors.WithCode(code.ErrInvalidArgument, "资源ID格式错误"))
		return
	}

	foundResource, err := h.queryer.GetResourceByID(c.Request.Context(), resourceDomain.NewResourceID(resourceID.Uint64()))
	if err != nil {
		handleError(c, err)
		return
	}

	success(c, h.toResourceResponse(foundResource))
}

// GetResourceByKey 根据键获取资源
// @Summary 根据键获取资源
// @Tags Authorization-Resources
// @Produce json
// @Param key path string true "资源键"
// @Success 200 {object} dto.Response{data=dto.ResourceResponse}
// @Router /authz/resources/key/{key} [get]
func (h *ResourceHandler) GetResourceByKey(c *gin.Context) {
	key := c.Param("key")

	foundResource, err := h.queryer.GetResourceByKey(c.Request.Context(), key)
	if err != nil {
		handleError(c, err)
		return
	}

	success(c, h.toResourceResponse(foundResource))
}

// ListResources 列出资源
// @Summary 列出资源
// @Tags Authorization-Resources
// @Produce json
// @Param app_name query string false "应用名称"
// @Param domain query string false "域"
// @Param type query string false "类型"
// @Param offset query int false "偏移量" default(0)
// @Param limit query int false "每页数量" default(10)
// @Success 200 {object} dto.ListResponse{data=[]dto.ResourceResponse}
// @Router /authz/resources [get]
func (h *ResourceHandler) ListResources(c *gin.Context) {
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	appName := c.Query("app_name")
	domain := c.Query("domain")
	typ := c.Query("type")

	query := resourceDomain.ListResourcesQuery{
		AppName: appName,
		Domain:  domain,
		Type:    typ,
		Offset:  offset,
		Limit:   limit,
	}

	result, err := h.queryer.ListResources(c.Request.Context(), query)
	if err != nil {
		handleError(c, err)
		return
	}

	resources := make([]dto.ResourceResponse, 0, len(result.Resources))
	for _, r := range result.Resources {
		resources = append(resources, h.toResourceResponse(r))
	}

	successList(c, resources, result.Total, offset, limit)
}

// ValidateAction 验证资源动作
// @Summary 验证资源动作
// @Tags Authorization-Resources
// @Accept json
// @Produce json
// @Param request body dto.ValidateActionRequest true "验证动作请求"
// @Success 200 {object} dto.Response{data=dto.ValidateActionResponse}
// @Router /authz/resources/validate-action [post]
func (h *ResourceHandler) ValidateAction(c *gin.Context) {
	var req dto.ValidateActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleError(c, errors.WithCode(code.ErrBind, "请求参数错误: %v", err))
		return
	}

	valid, err := h.queryer.ValidateAction(c.Request.Context(), req.ResourceKey, req.Action)
	if err != nil {
		handleError(c, err)
		return
	}

	success(c, dto.ValidateActionResponse{
		Valid: valid,
	})
}

// toResourceResponse 转换为响应对象
func (h *ResourceHandler) toResourceResponse(r *resourceDomain.Resource) dto.ResourceResponse {
	return dto.ResourceResponse{
		ID:          meta.FromUint64(r.ID.Uint64()),
		Key:         r.Key,
		DisplayName: r.DisplayName,
		AppName:     r.AppName,
		Domain:      r.Domain,
		Type:        r.Type,
		Actions:     r.Actions,
		Description: r.Description,
	}
}
