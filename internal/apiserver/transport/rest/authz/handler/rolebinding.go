// Package handler 角色分配处理器
package handler

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	bindingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/rolebinding"
	bindingDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authz/dto"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/gin-gonic/gin"
)

// RoleBindingHandler 角色分配处理器
type RoleBindingHandler struct {
	commander roleBindingCommander
	queryer   bindingApp.Directory
}

type roleBindingCommander interface {
	Grant(ctx context.Context, cmd bindingApp.GrantCommand) (*bindingDomain.Binding, error)
	Revoke(ctx context.Context, cmd bindingApp.RevokeCommand) error
	RevokeByID(ctx context.Context, cmd bindingApp.RevokeByIDCommand) error
}

// NewRoleBindingHandler 创建角色分配处理器
func NewRoleBindingHandler(commander roleBindingCommander, queryer bindingApp.Directory) *RoleBindingHandler {
	return &RoleBindingHandler{
		commander: commander,
		queryer:   queryer,
	}
}

// convertToSubjectType 将字符串转换为 SubjectType
func convertToSubjectType(s string) (bindingDomain.SubjectType, error) {
	switch s {
	case "user":
		return bindingDomain.SubjectTypeUser, nil
	default:
		return "", errors.WithCode(code.ErrInvalidArgument, "无效的主体类型: %s", s)
	}
}

// GrantRole 授予角色
// @Summary 授予角色
// @Tags Authorization-Assignments
// @Accept json
// @Produce json
// @Param request body dto.GrantRequest true "授予角色请求"
// @Success 200 {object} dto.Response{data=dto.AssignmentResponse}
// @Router /authz/assignments/grant [post]
func (h *RoleBindingHandler) GrantRoleBinding(c *gin.Context) {
	var req dto.GrantRequest
	if !bindJSON(c, &req) {
		return
	}

	tenantID, err := getTenantID(c)
	if err != nil {
		handleError(c, err)
		return
	}
	grantedBy, err := getUserID(c)
	if err != nil {
		handleError(c, err)
		return
	}

	subjectType, err := convertToSubjectType(req.SubjectType)
	if err != nil {
		handleError(c, err)
		return
	}

	cmd := bindingApp.GrantCommand{
		SubjectType: subjectType,
		SubjectID:   req.SubjectID,
		RoleID:      req.RoleID,
		TenantID:    tenantID,
		GrantedBy:   grantedBy.String(),
	}

	grantedBinding, err := h.commander.Grant(c.Request.Context(), cmd)
	if err != nil {
		handleError(c, err)
		return
	}

	success(c, h.toAssignmentResponse(grantedBinding))
}

// RevokeRole 撤销角色
// @Summary 撤销角色
// @Tags Authorization-Assignments
// @Accept json
// @Produce json
// @Param request body dto.RevokeRequest true "撤销角色请求"
// @Success 200 {object} dto.Response
// @Router /authz/assignments/revoke [post]
func (h *RoleBindingHandler) RevokeRoleBinding(c *gin.Context) {
	var req dto.RevokeRequest
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

	subjectType, err := convertToSubjectType(req.SubjectType)
	if err != nil {
		handleError(c, err)
		return
	}

	cmd := bindingApp.RevokeCommand{
		SubjectType: subjectType,
		SubjectID:   req.SubjectID,
		RoleID:      req.RoleID,
		TenantID:    tenantID,
		ChangedBy:   changedBy.String(),
		Reason:      req.Reason,
	}

	err = h.commander.Revoke(c.Request.Context(), cmd)
	if err != nil {
		handleError(c, err)
		return
	}

	successNoContent(c)
}

// RevokeRoleBindingByID 根据分配ID撤销角色
// @Summary 根据分配ID撤销角色
// @Tags Authorization-Assignments
// @Param id path string true "分配ID"
// @Success 200 {object} dto.Response
// @Router /authz/assignments/{id} [delete]
func (h *RoleBindingHandler) RevokeRoleBindingByID(c *gin.Context) {
	bindingID, ok := parseIDParam(c, "id", "分配ID格式错误")
	if !ok {
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

	cmd := bindingApp.RevokeByIDCommand{
		BindingID: bindingDomain.NewBindingID(bindingID.Uint64()),
		TenantID:  tenantID,
		ChangedBy: changedBy.String(),
	}

	err = h.commander.RevokeByID(c.Request.Context(), cmd)
	if err != nil {
		handleError(c, err)
		return
	}

	successNoContent(c)
}

// ListRoleBindingsBySubject 列出主体的角色分配
// @Summary 列出主体的角色分配
// @Tags Authorization-Assignments
// @Produce json
// @Param subject_type query string true "主体类型" Enums(user)
// @Param subject_id query string true "主体ID"
// @Success 200 {object} dto.Response{data=[]dto.AssignmentResponse}
// @Router /authz/assignments/subject [get]
func (h *RoleBindingHandler) ListRoleBindingsBySubject(c *gin.Context) {
	subjectTypeStr := c.Query("subject_type")
	subjectID, err := meta.ParseID(c.Query("subject_id"))

	if subjectTypeStr == "" || subjectID.IsZero() || err != nil {
		handleError(c, errors.WithCode(code.ErrInvalidArgument, "subject_type 和 subject_id 不能为空"))
		return
	}

	tenantID, err := getTenantID(c)
	if err != nil {
		handleError(c, err)
		return
	}

	subjectType, err := convertToSubjectType(subjectTypeStr)
	if err != nil {
		handleError(c, err)
		return
	}

	query := bindingApp.ListBySubjectQuery{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		TenantID:    tenantID,
	}

	result, err := h.queryer.ListBySubject(c.Request.Context(), query)
	if err != nil {
		handleError(c, err)
		return
	}

	bindings := make([]dto.AssignmentResponse, 0, len(result))
	for _, a := range result {
		bindings = append(bindings, h.toAssignmentResponse(a))
	}

	success(c, bindings)
}

// ListRoleBindingsByRole 列出角色的分配记录
// @Summary 列出角色的分配记录
// @Tags Authorization-Assignments
// @Produce json
// @Param id path string true "角色ID"
// @Success 200 {object} dto.Response{data=[]dto.AssignmentResponse}
// @Router /authz/roles/{id}/assignments [get]
func (h *RoleBindingHandler) ListRoleBindingsByRole(c *gin.Context) {
	roleID, ok := parseIDParam(c, "id", "角色ID格式错误")
	if !ok {
		return
	}

	tenantID, err := getTenantID(c)
	if err != nil {
		handleError(c, err)
		return
	}

	query := bindingApp.ListByRoleQuery{
		RoleID:   roleID,
		TenantID: tenantID,
	}

	result, err := h.queryer.ListByRole(c.Request.Context(), query)
	if err != nil {
		handleError(c, err)
		return
	}

	bindings := make([]dto.AssignmentResponse, 0, len(result))
	for _, a := range result {
		bindings = append(bindings, h.toAssignmentResponse(a))
	}

	success(c, bindings)
}
