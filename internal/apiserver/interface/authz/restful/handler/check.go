package handler

import (
	"github.com/FangcunMount/component-base/pkg/errors"
	assignmentDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/assignment"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/internal/apiserver/interface/authz/restful/dto"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/gin-gonic/gin"
)

// CheckHandler PDP（策略判定）HTTP 入口。
type CheckHandler struct {
	casbin policyDomain.CasbinAdapter
}

// NewCheckHandler 创建判定处理器。
func NewCheckHandler(casbin policyDomain.CasbinAdapter) *CheckHandler {
	return &CheckHandler{casbin: casbin}
}

// Check 对单条 (subject, domain, object, action) 执行 Casbin Enforce。
// @Summary 策略判定（Enforce）
// @Tags Authorization-Policies
// @Accept json
// @Produce json
// @Param request body dto.CheckRequest true "判定请求"
// @Success 200 {object} dto.Response{data=dto.CheckResponse}
// @Router /authz/check [post]
func (h *CheckHandler) Check(c *gin.Context) {
	if h.casbin == nil {
		handleError(c, errors.WithCode(code.ErrInternalServerError, "authorization engine not available"))
		return
	}

	var req dto.CheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleError(c, errors.WithCode(code.ErrBind, "请求参数错误: %v", err))
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
	allowed, err := h.casbin.Enforce(c.Request.Context(), sub, dom, req.Object, req.Action)
	if err != nil {
		handleError(c, err)
		return
	}

	success(c, dto.CheckResponse{Allowed: allowed})
}

func resolveSubject(c *gin.Context, req dto.CheckRequest) (string, bool) {
	if req.SubjectID != "" && req.SubjectType != "" {
		st := assignmentDomain.SubjectType(req.SubjectType)
		if st == "" {
			return "", false
		}
		return st.String() + ":" + req.SubjectID, true
	}
	uid, err := getUserID(c)
	if err != nil || uid == "" {
		return "", false
	}
	return "user:" + uid, true
}
