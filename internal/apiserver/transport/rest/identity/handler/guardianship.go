package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	appguard "github.com/FangcunMount/iam/internal/apiserver/application/uc/guardianship"
	requestdto "github.com/FangcunMount/iam/internal/apiserver/transport/rest/identity/request"
	responsedto "github.com/FangcunMount/iam/internal/apiserver/transport/rest/identity/response"
	"github.com/FangcunMount/iam/internal/pkg/code"
	_ "github.com/FangcunMount/iam/pkg/core" // imported for swagger
)

// GuardianshipHandler 监护关系 REST 处理器
type GuardianshipHandler struct {
	*BaseHandler
	guardAccess appguard.GuardianshipAccessApplicationService
}

// NewGuardianshipHandler 创建监护处理器
func NewGuardianshipHandler(
	guardAccess appguard.GuardianshipAccessApplicationService,
) *GuardianshipHandler {
	return &GuardianshipHandler{
		BaseHandler: NewBaseHandler(),
		guardAccess: guardAccess,
	}
}

// Grant 授予监护关系
// @Summary 授予监护关系
// @Description 将用户设置为儿童的监护人
// @Tags Identity-Guardianship
// @Accept json
// @Produce json
// @Param request body requestdto.GuardianGrantRequest true "授予监护请求"
// @Success 201 {object} responsedto.GuardianshipResponse "授予成功"
// @Failure 400 {object} core.ErrResponse "参数错误"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 409 {object} core.ErrResponse "监护关系已存在"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /identity/guardians/grant [post]
// @Security BearerAuth
func (h *GuardianshipHandler) Grant(c *gin.Context) {
	var req requestdto.GuardianGrantRequest
	if err := h.BindJSON(c, &req); err != nil {
		h.Error(c, err)
		return
	}

	currentUserID, ok := h.GetUserID(c)
	if !ok {
		h.ErrorWithCode(c, code.ErrTokenInvalid, "user id not found in context")
		return
	}

	result, err := h.guardAccess.GrantForCurrentUser(c.Request.Context(), currentUserID, appguard.AddGuardianDTO{
		UserID:   req.UserID,
		ChildID:  req.ChildID,
		Relation: req.Relation,
	})
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Created(c, guardResultToResponse(result))
}

// List 查询监护关系
// @Summary 查询监护关系
// @Description 查询用户或儿童的监护关系列表
// @Tags Identity-Guardianship
// @Accept json
// @Produce json
// @Param user_id query string false "用户 ID"
// @Param child_id query string false "儿童 ID"
// @Param active query boolean false "是否仅查询活跃的监护关系"
// @Param offset query int false "偏移量" default(0)
// @Param limit query int false "每页数量" default(20)
// @Success 200 {object} responsedto.GuardianshipPageResponse "查询成功"
// @Failure 400 {object} core.ErrResponse "参数错误"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /identity/guardians [get]
// @Security BearerAuth
func (h *GuardianshipHandler) List(c *gin.Context) {
	var req requestdto.GuardianshipListQuery
	if err := h.BindQuery(c, &req); err != nil {
		h.Error(c, err)
		return
	}

	currentUserID, ok := h.GetUserID(c)
	if !ok {
		h.ErrorWithCode(c, code.ErrTokenInvalid, "user id not found in context")
		return
	}

	results, err := h.guardAccess.ListForCurrentUser(c.Request.Context(), currentUserID, appguard.ListGuardianshipsDTO{
		UserID:  req.UserID,
		ChildID: req.ChildID,
		Active:  req.Active,
	})
	if err != nil {
		h.Error(c, err)
		return
	}

	total := len(results)
	items := make([]responsedto.GuardianshipResponse, 0, total)
	for _, g := range results {
		if g == nil {
			continue
		}
		items = append(items, guardResultToResponse(g))
	}

	sliced := sliceGuardianships(items, req.Offset, req.Limit)

	h.Success(c, responsedto.GuardianshipPageResponse{
		Total:  total,
		Limit:  req.Limit,
		Offset: req.Offset,
		Items:  sliced,
	})
}

// ========== 辅助函数 ==========

// guardResultToResponse 将应用服务返回的 GuardianshipResult 转换为响应 DTO
func guardResultToResponse(result *appguard.GuardianshipResult) responsedto.GuardianshipResponse {
	if result == nil {
		return responsedto.GuardianshipResponse{}
	}

	resp := responsedto.GuardianshipResponse{
		ID:       result.ID,
		UserID:   result.UserID,
		ChildID:  result.ChildID,
		Relation: result.Relation,
		Since:    parseGuardTime(result.EstablishedAt),
	}
	if revokedAt := parseOptionalTime(result.RevokedAt); revokedAt != nil {
		resp.RevokedAt = revokedAt
	}

	return resp
}

// parseGuardTime 解析时间字符串
func parseGuardTime(timeStr string) time.Time {
	if timeStr == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Time{}
	}
	return t
}

// sliceGuardianships 分页切片
func sliceGuardianships(items []responsedto.GuardianshipResponse, offset, limit int) []responsedto.GuardianshipResponse {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []responsedto.GuardianshipResponse{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}
