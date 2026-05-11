package handler

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	authzapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/authorization"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/decision"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authz/dto"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/gin-gonic/gin"
)

type authorizationChecker interface {
	Check(ctx context.Context, cmd authzapp.CheckCommand) (decision.Decision, error)
}

// CheckHandler PDP（策略判定）HTTP 入口。
type CheckHandler struct {
	checker authorizationChecker
}

// NewCheckHandler 创建判定处理器。
func NewCheckHandler(checker authorizationChecker) *CheckHandler {
	return &CheckHandler{checker: checker}
}

// Check 对单条 (subject, domain, object, action) 执行 授权判定。
// @Summary 策略判定（Enforce）
// @Tags Authorization-Policies
// @Accept json
// @Produce json
// @Param request body dto.CheckRequest true "判定请求"
// @Success 200 {object} dto.Response{data=dto.CheckResponse}
// @Router /authz/check [post]
func (h *CheckHandler) Check(c *gin.Context) {
	if h.checker == nil {
		handleError(c, errors.WithCode(code.ErrInternalServerError, "authorization engine not available"))
		return
	}

	var req dto.CheckRequest
	if !bindJSON(c, &req) {
		return
	}

	sub, ok := resolveSubject(c, req)
	if !ok {
		handleError(c, errors.WithCode(code.ErrUnauthorized, "subject required: authenticate or pass subject_type and subject_id"))
		return
	}

	dom, err := getTenantID(c)
	if err != nil {
		handleError(c, err)
		return
	}
	scope, ok := parseScope(c, req.ScopeType, req.ScopeValue)
	if !ok {
		return
	}
	decision, err := h.checker.Check(c.Request.Context(), authzapp.CheckCommand{
		Subject:     sub,
		TenantID:    dom,
		ResourceKey: req.Object,
		Action:      req.Action,
		ObjectScope: scope,
	})
	if err != nil {
		handleError(c, err)
		return
	}

	success(c, dto.CheckResponse{
		Allowed:       decision.Allowed,
		Reason:        string(decision.Reason),
		DenyCode:      decision.DenyCode,
		PolicyVersion: decision.PolicyVersion,
	})
}

func resolveSubject(c *gin.Context, req dto.CheckRequest) (subject.Ref, bool) {
	if !req.SubjectID.IsZero() && req.SubjectType != "" {
		subjectRef, err := subject.NewRef(subject.Type(req.SubjectType), req.SubjectID)
		if err != nil {
			return subject.Ref{}, false
		}
		return subjectRef, true
	}
	uid, err := getUserID(c)
	if err != nil || uid.IsZero() {
		return subject.Ref{}, false
	}
	subjectRef, err := subject.NewUserRef(uid)
	if err != nil {
		return subject.Ref{}, false
	}
	return subjectRef, true
}
