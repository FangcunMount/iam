// Package handler 策略管理处理器
package handler

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	policyApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policy"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policylint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authz/dto"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/gin-gonic/gin"
)

type policyLinter interface {
	Lint(ctx context.Context) (policylint.Report, error)
}

// PolicyHandler 策略处理器
type PolicyHandler struct {
	commander policyApp.PermissionCommands
	queryer   policyApp.PermissionReader
	linter    policyLinter
}

// NewPolicyHandler 创建策略处理器
func NewPolicyHandler(commander policyApp.PermissionCommands, queryer policyApp.PermissionReader, linters ...policyLinter) *PolicyHandler {
	var linter policyLinter
	if len(linters) > 0 {
		linter = linters[0]
	}
	return &PolicyHandler{
		commander: commander,
		queryer:   queryer,
		linter:    linter,
	}
}

// AddPermission 添加权限
// @Summary 添加权限
// @Tags Authorization-Policies
// @Accept json
// @Produce json
// @Param request body dto.AddPermissionRequest true "添加策略请求"
// @Success 200 {object} dto.Response
// @Router /authz/policies [post]
func (h *PolicyHandler) AddPermission(c *gin.Context) {
	var req dto.AddPermissionRequest
	if !bindJSON(c, &req) {
		return
	}

	tenantID, err := getTenantID(c)
	if err != nil {
		handleError(c, err)
		return
	}
	changedBy, err := getUserID(c)
	if err != nil {
		handleError(c, err)
		return
	}

	roleID := req.RoleID
	resourceID := req.ResourceID

	if roleID.IsZero() {
		handleError(c, errors.WithCode(code.ErrInvalidArgument, "角色ID不能为空"))
		return
	}
	if resourceID.IsZero() {
		handleError(c, errors.WithCode(code.ErrInvalidArgument, "资源ID不能为空"))
		return
	}
	scope, ok := parseScope(c, req.ScopeType, req.ScopeValue)
	if !ok {
		return
	}

	cmd, err := policyApp.NewAddPermissionCommand(roleID, resource.NewResourceID(resourceID.Uint64()), req.Action, scope, tenantID, changedBy.String(), req.Reason)
	if err != nil {
		handleError(c, err)
		return
	}

	err = h.commander.AddPermission(c.Request.Context(), cmd)
	if err != nil {
		handleError(c, err)
		return
	}

	successNoContent(c)
}

// RemovePermission 移除权限
// @Summary 移除权限
// @Tags Authorization-Policies
// @Accept json
// @Produce json
// @Param request body dto.RemovePermissionRequest true "移除策略请求"
// @Success 200 {object} dto.Response
// @Router /authz/policies [delete]
func (h *PolicyHandler) RemovePermission(c *gin.Context) {
	var req dto.RemovePermissionRequest
	if !bindJSON(c, &req) {
		return
	}

	tenantID, err := getTenantID(c)
	if err != nil {
		handleError(c, err)
		return
	}
	changedBy, err := getUserID(c)
	if err != nil {
		handleError(c, err)
		return
	}

	roleID := req.RoleID
	resourceID := req.ResourceID

	if roleID.IsZero() {
		handleError(c, errors.WithCode(code.ErrInvalidArgument, "角色ID不能为空"))
		return
	}
	if resourceID.IsZero() {
		handleError(c, errors.WithCode(code.ErrInvalidArgument, "资源ID不能为空"))
		return
	}
	scope, ok := parseScope(c, req.ScopeType, req.ScopeValue)
	if !ok {
		return
	}

	cmd, err := policyApp.NewRemovePermissionCommand(roleID, resource.NewResourceID(resourceID.Uint64()), req.Action, scope, tenantID, changedBy.String(), req.Reason)
	if err != nil {
		handleError(c, err)
		return
	}

	err = h.commander.RemovePermission(c.Request.Context(), cmd)
	if err != nil {
		handleError(c, err)
		return
	}

	successNoContent(c)
}

// GetPoliciesByRole 获取角色的策略列表
// @Summary 获取角色的策略列表
// @Tags Authorization-Policies
// @Produce json
// @Param id path string true "角色ID"
// @Success 200 {object} dto.Response{data=[]dto.PermissionResponse}
// @Router /authz/roles/{id}/policies [get]
func (h *PolicyHandler) GetPoliciesByRole(c *gin.Context) {
	roleID, ok := parseIDParam(c, "id", "角色ID格式错误")
	if !ok {
		return
	}

	tenantID, err := getTenantID(c)
	if err != nil {
		handleError(c, err)
		return
	}

	query := policyApp.RolePermissionsQuery{
		RoleID:   roleID,
		TenantID: tenantID,
	}

	rules, err := h.queryer.GetPermissionsForRole(c.Request.Context(), query)
	if err != nil {
		handleError(c, err)
		return
	}

	success(c, toPermissionResponses(rules))
}

// GetCurrentVersion 获取当前策略版本
// @Summary 获取当前策略版本
// @Tags Authorization-Policies
// @Produce json
// @Success 200 {object} dto.Response{data=dto.PolicyVersionResponse}
// @Router /authz/policies/version [get]
func (h *PolicyHandler) GetCurrentVersion(c *gin.Context) {
	tenantID, err := getTenantID(c)
	if err != nil {
		handleError(c, err)
		return
	}

	query := policyApp.CurrentVersionQuery{
		TenantID: tenantID,
	}

	version, err := h.queryer.GetCurrentVersion(c.Request.Context(), query)
	if err != nil {
		handleError(c, err)
		return
	}

	if version == nil {
		success(c, emptyPolicyVersionResponse(tenantID))
		return
	}

	success(c, toPolicyVersionResponse(version))
}

// LintPolicies runs the read-only authorization policy linter.
// @Summary 检查授权策略事实
// @Tags Authorization-Policies
// @Produce json
// @Success 200 {object} dto.Response{data=dto.PolicyLintResponse}
// @Router /authz/policies/lint [get]
func (h *PolicyHandler) LintPolicies(c *gin.Context) {
	if h.linter == nil {
		handleError(c, errors.WithCode(code.ErrInternalServerError, "authorization policy linter not available"))
		return
	}
	report, err := h.linter.Lint(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}
	success(c, toPolicyLintResponse(report))
}
