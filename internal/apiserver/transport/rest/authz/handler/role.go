// Package handler 角色管理处理器
package handler

import (
	"github.com/FangcunMount/component-base/pkg/errors"
	roleDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/internal/apiserver/transport/rest/authz/dto"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/gin-gonic/gin"
)

// RoleHandler 角色处理器
//
// 依赖倒置原则：Handler 依赖 driving 接口，不依赖具体实现
type RoleHandler struct {
	commander roleDomain.Commander // 命令服务（写操作）
	queryer   roleDomain.Queryer   // 查询服务（读操作）
}

// NewRoleHandler 创建角色处理器
func NewRoleHandler(
	commander roleDomain.Commander,
	queryer roleDomain.Queryer,
) *RoleHandler {
	return &RoleHandler{
		commander: commander,
		queryer:   queryer,
	}
}

// CreateRole 创建角色
// @Summary 创建角色
// @Tags Authorization-Roles
// @Accept json
// @Produce json
// @Param request body dto.CreateRoleRequest true "创建角色请求"
// @Success 200 {object} dto.Response{data=dto.RoleResponse}
// @Router /authz/roles [post]
func (h *RoleHandler) CreateRole(c *gin.Context) {
	var req dto.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleError(c, errors.WithCode(code.ErrBind, "请求参数错误: %v", err))
		return
	}

	tenantID, err := getTenantID(c)
	if err != nil {
		handleError(c, err)
		return
	}

	cmd := roleDomain.CreateRoleCommand{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		TenantID:    tenantID,
		Description: req.Description,
	}

	createdRole, err := h.commander.CreateRole(c.Request.Context(), cmd)
	if err != nil {
		handleError(c, err)
		return
	}

	success(c, h.toRoleResponse(createdRole))
}

// UpdateRole 更新角色
// @Summary 更新角色
// @Tags Authorization-Roles
// @Accept json
// @Produce json
// @Param id path string true "角色ID"
// @Param request body dto.UpdateRoleRequest true "更新角色请求"
// @Success 200 {object} dto.Response{data=dto.RoleResponse}
// @Router /authz/roles/{id} [put]
func (h *RoleHandler) UpdateRole(c *gin.Context) {
	roleID, err := meta.ParseID(c.Param("id"))
	if err != nil {
		handleError(c, errors.WithCode(code.ErrInvalidArgument, "角色ID格式错误"))
		return
	}

	var req dto.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleError(c, errors.WithCode(code.ErrBind, "请求参数错误: %v", err))
		return
	}

	cmd := roleDomain.UpdateRoleCommand{
		ID:          roleID,
		DisplayName: &req.DisplayName,
		Description: &req.Description,
	}

	updatedRole, err := h.commander.UpdateRole(c.Request.Context(), cmd)
	if err != nil {
		handleError(c, err)
		return
	}

	success(c, h.toRoleResponse(updatedRole))
}

// DeleteRole 删除角色
// @Summary 删除角色
// @Tags Authorization-Roles
// @Param id path string true "角色ID"
// @Success 200 {object} dto.Response
// @Router /authz/roles/{id} [delete]
func (h *RoleHandler) DeleteRole(c *gin.Context) {
	roleID, err := meta.ParseID(c.Param("id"))
	if err != nil {
		handleError(c, errors.WithCode(code.ErrInvalidArgument, "角色ID格式错误"))
		return
	}

	err = h.commander.DeleteRole(c.Request.Context(), roleID)
	if err != nil {
		handleError(c, err)
		return
	}

	successNoContent(c)
}

// GetRole 获取角色详情
// @Summary 获取角色详情
// @Tags Authorization-Roles
// @Produce json
// @Param id path string true "角色ID"
// @Success 200 {object} dto.Response{data=dto.RoleResponse}
// @Router /authz/roles/{id} [get]
func (h *RoleHandler) GetRole(c *gin.Context) {
	roleID, err := meta.ParseID(c.Param("id"))
	if err != nil {
		handleError(c, errors.WithCode(code.ErrInvalidArgument, "角色ID格式错误"))
		return
	}

	foundRole, err := h.queryer.GetRoleByID(c.Request.Context(), meta.FromUint64(uint64(roleID)))
	if err != nil {
		handleError(c, err)
		return
	}

	success(c, h.toRoleResponse(foundRole))
}

// ListRoles 列出角色
// @Summary 列出角色
// @Tags Authorization-Roles
// @Produce json
// @Param offset query int false "偏移量" default(0)
// @Param limit query int false "每页数量" default(10)
// @Success 200 {object} dto.ListResponse{data=[]dto.RoleResponse}
// @Router /authz/roles [get]
func (h *RoleHandler) ListRoles(c *gin.Context) {
	var query dto.ListRoleQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		handleError(c, errors.WithCode(code.ErrBind, "请求参数错误: %v", err))
		return
	}

	tenantID, err := getTenantID(c)
	if err != nil {
		handleError(c, err)
		return
	}

	listQuery := roleDomain.ListRolesQuery{
		TenantID: tenantID,
		Offset:   query.Offset,
		Limit:    query.Limit,
	}

	result, err := h.queryer.ListRoles(c.Request.Context(), listQuery)
	if err != nil {
		handleError(c, err)
		return
	}

	roles := make([]dto.RoleResponse, 0, len(result.Roles))
	for _, r := range result.Roles {
		roles = append(roles, h.toRoleResponse(r))
	}

	successList(c, roles, result.Total, query.Offset, query.Limit)
}
