// Package handler 角色分配处理器
package handler

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	assignmentApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignment"
	assignmentDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authz/dto"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/gin-gonic/gin"
)

// AssignmentHandler 角色赋权处理器。
type AssignmentHandler struct {
	commander assignmentCommander
	queryer   assignmentApp.Directory
}

type assignmentCommander interface {
	Grant(ctx context.Context, cmd assignmentApp.GrantCommand) (*assignmentDomain.Assignment, error)
	Revoke(ctx context.Context, cmd assignmentApp.RevokeCommand) error
	RevokeByID(ctx context.Context, cmd assignmentApp.RevokeByIDCommand) error
}

// NewAssignmentHandler 创建角色赋权处理器。
func NewAssignmentHandler(commander assignmentCommander, queryer assignmentApp.Directory) *AssignmentHandler {
	return &AssignmentHandler{
		commander: commander,
		queryer:   queryer,
	}
}

// convertToSubjectType 将字符串转换为 SubjectType
func convertToSubjectType(s string) (assignmentDomain.SubjectType, error) {
	switch s {
	case "user":
		return assignmentDomain.SubjectTypeUser, nil
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
// @Failure 503 {object} dto.ErrorResponse "Authorization policy unavailable (103002)"
// @Router /v3/authz/assignments/grant [post]
func (h *AssignmentHandler) GrantAssignment(c *gin.Context) {
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

	cmd, err := assignmentApp.NewGrantCommand(subjectType, req.SubjectID, req.RoleID, tenantID, grantedBy.String())
	if err != nil {
		handleError(c, err)
		return
	}

	grantedAssignment, err := h.commander.Grant(c.Request.Context(), cmd)
	if err != nil {
		handleError(c, err)
		return
	}

	success(c, h.toAssignmentResponse(grantedAssignment))
}

// RevokeRole 撤销角色
// @Summary 撤销角色
// @Tags Authorization-Assignments
// @Accept json
// @Produce json
// @Param request body dto.RevokeRequest true "撤销角色请求"
// @Success 200 {object} dto.Response
// @Failure 503 {object} dto.ErrorResponse "Authorization policy unavailable (103002)"
// @Router /v3/authz/assignments/revoke [post]
func (h *AssignmentHandler) RevokeAssignment(c *gin.Context) {
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

	cmd, err := assignmentApp.NewRevokeCommand(subjectType, req.SubjectID, req.RoleID, tenantID, changedBy.String(), req.Reason)
	if err != nil {
		handleError(c, err)
		return
	}

	err = h.commander.Revoke(c.Request.Context(), cmd)
	if err != nil {
		handleError(c, err)
		return
	}

	successNoContent(c)
}

// RevokeAssignmentByID 根据分配 ID 撤销角色。
// @Summary 根据分配ID撤销角色
// @Tags Authorization-Assignments
// @Param id path string true "分配ID"
// @Success 200 {object} dto.Response
// @Failure 503 {object} dto.ErrorResponse "Authorization policy unavailable (103002)"
// @Router /v3/authz/assignments/{id} [delete]
func (h *AssignmentHandler) RevokeAssignmentByID(c *gin.Context) {
	assignmentID, ok := parseIDParam(c, "id", "分配ID格式错误")
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

	cmd, err := assignmentApp.NewRevokeByIDCommand(assignmentDomain.NewAssignmentID(assignmentID.Uint64()), tenantID, changedBy.String(), "")
	if err != nil {
		handleError(c, err)
		return
	}

	err = h.commander.RevokeByID(c.Request.Context(), cmd)
	if err != nil {
		handleError(c, err)
		return
	}

	successNoContent(c)
}

// ListAssignmentsBySubject 列出主体的角色分配。
// @Summary 列出主体的角色分配
// @Tags Authorization-Assignments
// @Produce json
// @Param subject_type query string true "主体类型" Enums(user)
// @Param subject_id query string true "主体ID"
// @Success 200 {object} dto.Response{data=[]dto.AssignmentResponse}
// @Failure 503 {object} dto.ErrorResponse "Authorization policy unavailable (103002)"
// @Router /v3/authz/assignments/subject [get]
func (h *AssignmentHandler) ListAssignmentsBySubject(c *gin.Context) {
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

	query := assignmentApp.ListBySubjectQuery{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		TenantID:    tenantID,
	}

	result, err := h.queryer.ListBySubject(c.Request.Context(), query)
	if err != nil {
		handleError(c, err)
		return
	}

	assignments := make([]dto.AssignmentResponse, 0, len(result))
	for _, a := range result {
		assignments = append(assignments, h.toAssignmentResponse(a))
	}

	success(c, assignments)
}

// ListAssignmentsByRole 列出角色的分配记录。
// @Summary 列出角色的分配记录
// @Tags Authorization-Assignments
// @Produce json
// @Param id path string true "角色ID"
// @Success 200 {object} dto.Response{data=[]dto.AssignmentResponse}
// @Failure 503 {object} dto.ErrorResponse "Authorization policy unavailable (103002)"
// @Router /v3/authz/roles/{id}/assignments [get]
func (h *AssignmentHandler) ListAssignmentsByRole(c *gin.Context) {
	roleID, ok := parseIDParam(c, "id", "角色ID格式错误")
	if !ok {
		return
	}

	tenantID, err := getTenantID(c)
	if err != nil {
		handleError(c, err)
		return
	}

	query := assignmentApp.ListByRoleQuery{
		RoleID:   roleID,
		TenantID: tenantID,
	}

	result, err := h.queryer.ListByRole(c.Request.Context(), query)
	if err != nil {
		handleError(c, err)
		return
	}

	assignments := make([]dto.AssignmentResponse, 0, len(result))
	for _, a := range result {
		assignments = append(assignments, h.toAssignmentResponse(a))
	}

	success(c, assignments)
}
