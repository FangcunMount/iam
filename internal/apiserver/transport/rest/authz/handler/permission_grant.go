package handler

import (
	"context"

	permissionGrantApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/permissiongrant"
	permissionGrantDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authz/dto"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/gin-gonic/gin"
)

type permissionGrantService interface {
	Create(context.Context, permissionGrantApp.CreateCommand) (*permissionGrantDomain.Grant, error)
	Revoke(context.Context, permissionGrantApp.RevokeCommand) error
	ListByRole(context.Context, meta.ID, string) ([]*permissionGrantDomain.Grant, error)
}

type PermissionGrantHandler struct{ service permissionGrantService }

func NewPermissionGrantHandler(service permissionGrantService) *PermissionGrantHandler {
	return &PermissionGrantHandler{service: service}
}

// CreateGrant creates an immutable typed permission grant.
// @Summary 创建 PermissionGrant
// @Tags Authorization-Grants
// @Accept json
// @Produce json
// @Param request body dto.CreatePermissionGrantRequest true "Grant"
// @Success 200 {object} dto.Response{data=dto.PermissionGrantResponse}
// @Router /v3/authz/grants [post]
func (h *PermissionGrantHandler) CreateGrant(c *gin.Context) {
	var req dto.CreatePermissionGrantRequest
	if !bindJSON(c, &req) {
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
	grant, err := h.service.Create(c.Request.Context(), permissionGrantApp.CreateCommand{
		TenantID: tenantID, RoleID: req.RoleID,
		ResourceID: resource.NewResourceID(req.ResourceID.Uint64()), Action: req.Action,
		Constraints: req.ConstraintSet, GrantedBy: userID.String(),
	})
	if err != nil {
		handleError(c, err)
		return
	}
	success(c, toPermissionGrantResponse(grant))
}

// RevokeGrant revokes an immutable permission grant.
// @Summary 撤销 PermissionGrant
// @Tags Authorization-Grants
// @Accept json
// @Produce json
// @Param id path string true "Grant ID"
// @Param request body dto.RevokePermissionGrantRequest false "Revoke"
// @Success 200 {object} dto.Response
// @Router /v3/authz/grants/{id} [delete]
func (h *PermissionGrantHandler) RevokeGrant(c *gin.Context) {
	grantID, ok := parseIDParam(c, "id", "Grant ID 格式错误")
	if !ok {
		return
	}
	var req dto.RevokePermissionGrantRequest
	if c.Request.ContentLength > 0 && !bindJSON(c, &req) {
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
	if err := h.service.Revoke(c.Request.Context(), permissionGrantApp.RevokeCommand{
		TenantID: tenantID, GrantID: grantID, RevokedBy: userID.String(), Reason: req.Reason,
	}); err != nil {
		handleError(c, err)
		return
	}
	successNoContent(c)
}

// ListRoleGrants lists grants for one role.
// @Summary 查询角色 PermissionGrant
// @Tags Authorization-Grants
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} dto.Response{data=[]dto.PermissionGrantResponse}
// @Router /v3/authz/roles/{id}/grants [get]
func (h *PermissionGrantHandler) ListRoleGrants(c *gin.Context) {
	roleID, ok := parseIDParam(c, "id", "角色 ID 格式错误")
	if !ok {
		return
	}
	tenantID, err := getTenantID(c)
	if err != nil {
		handleError(c, err)
		return
	}
	grants, err := h.service.ListByRole(c.Request.Context(), roleID, tenantID)
	if err != nil {
		handleError(c, err)
		return
	}
	result := make([]dto.PermissionGrantResponse, 0, len(grants))
	for _, grant := range grants {
		result = append(result, toPermissionGrantResponse(grant))
	}
	success(c, result)
}

func toPermissionGrantResponse(grant *permissionGrantDomain.Grant) dto.PermissionGrantResponse {
	if grant == nil {
		return dto.PermissionGrantResponse{}
	}
	return dto.PermissionGrantResponse{
		ID: grant.ID, TenantID: grant.TenantIDString(), RoleID: grant.RoleID,
		ResourceID: meta.FromUint64(grant.ResourceID.Uint64()), ResourcePattern: grant.ResourcePatternString(),
		Action: grant.ActionString(), ConstraintSet: grant.Constraints,
		GrantKey: grant.GrantKey, GrantedBy: grant.GrantedBy, Active: grant.IsActive(),
	}
}
