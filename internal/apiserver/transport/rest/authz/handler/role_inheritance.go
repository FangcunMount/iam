package handler

import (
	"context"

	roleInheritanceApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/roleinheritance"
	roleInheritanceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authz/dto"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/gin-gonic/gin"
)

type roleInheritanceService interface {
	Create(context.Context, roleInheritanceApp.CreateCommand) (*roleInheritanceDomain.Inheritance, error)
	Revoke(context.Context, roleInheritanceApp.RevokeCommand) error
	List(context.Context, string, meta.ID) ([]*roleInheritanceDomain.Inheritance, error)
}

type RoleInheritanceHandler struct{ service roleInheritanceService }

func NewRoleInheritanceHandler(service roleInheritanceService) *RoleInheritanceHandler {
	return &RoleInheritanceHandler{service: service}
}

// Create creates an active role-inheritance edge.
// @Summary 创建角色继承
// @Description 当前角色继承目标角色的全部有效能力；循环关系会被拒绝
// @ID createRoleInheritance
// @Tags Authorization-Role-Inheritances
// @Accept json
// @Produce json
// @Param request body dto.CreateRoleInheritanceRequest true "Role inheritance"
// @Success 200 {object} dto.Response{data=dto.RoleInheritanceResponse}
// @Failure 503 {object} dto.ErrorResponse "Authorization policy unavailable (103002)"
// @Router /v3/authz/role-inheritances [post]
func (h *RoleInheritanceHandler) Create(c *gin.Context) {
	var request dto.CreateRoleInheritanceRequest
	if !bindJSON(c, &request) {
		return
	}
	tenantID, err := getTenantID(c)
	if err != nil {
		handleError(c, err)
		return
	}
	userID, err := getUserID(c)
	if err != nil {
		handleError(c, err)
		return
	}
	inheritance, err := h.service.Create(c.Request.Context(), roleInheritanceApp.CreateCommand{
		TenantID: tenantID, RoleID: request.RoleID, InheritedRoleID: request.InheritedRoleID, GrantedBy: userID.String(),
	})
	if err != nil {
		handleError(c, err)
		return
	}
	success(c, toRoleInheritanceResponse(inheritance))
}

// Revoke revokes one role-inheritance edge.
// @Summary 撤销角色继承
// @Description 按继承关系 ID 撤销当前租户中的有效角色继承
// @ID revokeRoleInheritance
// @Tags Authorization-Role-Inheritances
// @Accept json
// @Produce json
// @Param id path string true "Role inheritance ID"
// @Param request body dto.RevokeRoleInheritanceRequest false "Revoke"
// @Success 200 {object} dto.Response
// @Failure 503 {object} dto.ErrorResponse "Authorization policy unavailable (103002)"
// @Router /v3/authz/role-inheritances/{id} [delete]
func (h *RoleInheritanceHandler) Revoke(c *gin.Context) {
	id, ok := parseIDParam(c, "id", "角色继承 ID 格式错误")
	if !ok {
		return
	}
	var request dto.RevokeRoleInheritanceRequest
	if c.Request.ContentLength > 0 && !bindJSON(c, &request) {
		return
	}
	tenantID, err := getTenantID(c)
	if err != nil {
		handleError(c, err)
		return
	}
	userID, err := getUserID(c)
	if err != nil {
		handleError(c, err)
		return
	}
	if err := h.service.Revoke(c.Request.Context(), roleInheritanceApp.RevokeCommand{
		TenantID: tenantID, ID: id, RevokedBy: userID.String(), Reason: request.Reason,
	}); err != nil {
		handleError(c, err)
		return
	}
	successNoContent(c)
}

// List lists active inheritance edges in the tenant.
// @Summary 查询角色继承
// @Description 查询当前租户中的有效角色继承，可按获得能力的角色过滤
// @ID listRoleInheritances
// @Tags Authorization-Role-Inheritances
// @Produce json
// @Param role_id query string false "Receiving role ID"
// @Success 200 {object} dto.Response{data=[]dto.RoleInheritanceResponse}
// @Failure 503 {object} dto.ErrorResponse "Authorization policy unavailable (103002)"
// @Router /v3/authz/role-inheritances [get]
func (h *RoleInheritanceHandler) List(c *gin.Context) {
	var query dto.ListRoleInheritanceQuery
	if !bindQuery(c, &query) {
		return
	}
	tenantID, err := getTenantID(c)
	if err != nil {
		handleError(c, err)
		return
	}
	items, err := h.service.List(c.Request.Context(), tenantID, query.RoleID)
	if err != nil {
		handleError(c, err)
		return
	}
	result := make([]dto.RoleInheritanceResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toRoleInheritanceResponse(item))
	}
	success(c, result)
}

func toRoleInheritanceResponse(inheritance *roleInheritanceDomain.Inheritance) dto.RoleInheritanceResponse {
	if inheritance == nil {
		return dto.RoleInheritanceResponse{}
	}
	return dto.RoleInheritanceResponse{
		ID: inheritance.ID, TenantID: inheritance.TenantIDString(), RoleID: inheritance.RoleID,
		InheritedRoleID: inheritance.InheritedRoleID, GrantedBy: inheritance.GrantedBy,
		GrantedAt: inheritance.GrantedAt, Active: inheritance.IsActive(),
	}
}
