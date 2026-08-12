// Package handler 角色管理处理器
package handler

import (
	roleApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authz/dto"
	"github.com/gin-gonic/gin"
)

// RoleHandler 角色处理器
//
// 依赖倒置原则：Handler 依赖 driving 接口，不依赖具体实现
type RoleHandler struct {
	commander roleApp.Catalog
	queryer   roleApp.Directory
}

// NewRoleHandler 创建角色处理器
func NewRoleHandler(
	commander roleApp.Catalog,
	queryer roleApp.Directory,
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
	if !bindJSON(c, &req) {
		return
	}

	tenantID, err := getTenantID(c)
	if err != nil {
		handleError(c, err)
		return
	}

	cmd, err := roleApp.NewCreateRoleCommand(req.Name, req.DisplayName, tenantID, req.Description)
	if err != nil {
		handleError(c, err)
		return
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
	roleID, ok := parseIDParam(c, "id", "角色ID格式错误")
	if !ok {
		return
	}

	var req dto.UpdateRoleRequest
	if !bindJSON(c, &req) {
		return
	}

	cmd, err := roleApp.NewUpdateRoleCommand(roleID, &req.DisplayName, &req.Description)
	if err != nil {
		handleError(c, err)
		return
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
	roleID, ok := parseIDParam(c, "id", "角色ID格式错误")
	if !ok {
		return
	}

	if err := h.commander.DeleteRole(c.Request.Context(), roleID); err != nil {
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
	roleID, ok := parseIDParam(c, "id", "角色ID格式错误")
	if !ok {
		return
	}

	foundRole, err := h.queryer.GetRoleByID(c.Request.Context(), roleID)
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
	if !bindQuery(c, &query) {
		return
	}

	tenantID, err := getTenantID(c)
	if err != nil {
		handleError(c, err)
		return
	}

	listQuery, err := roleApp.NewListRolesQuery(tenantID, query.Offset, query.Limit)
	if err != nil {
		handleError(c, err)
		return
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
