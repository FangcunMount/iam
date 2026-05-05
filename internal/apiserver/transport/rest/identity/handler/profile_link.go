package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	appprofilelink "github.com/FangcunMount/iam/v2/internal/apiserver/application/uc/profilelink"
	requestdto "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/identity/request"
	responsedto "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/identity/response"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/pkg/core"
)

var _ = core.ErrResponse{}

// ProfileLinkHandler 档案关系 REST 处理器
type ProfileLinkHandler struct {
	*BaseHandler
	profileLinkAccess appprofilelink.MyProfileLinks
}

// NewProfileLinkHandler 创建关系处理器
func NewProfileLinkHandler(
	profileLinkAccess appprofilelink.MyProfileLinks,
) *ProfileLinkHandler {
	return &ProfileLinkHandler{
		BaseHandler:       NewBaseHandler(),
		profileLinkAccess: profileLinkAccess,
	}
}

// Grant 授予档案关系
// @Summary 授予档案关系
// @Description 将用户设置为档案的关系用户
// @Tags Identity-ProfileLink
// @Accept json
// @Produce json
// @Param request body requestdto.ProfileLinkCreateRequest true "授予关系请求"
// @Success 201 {object} responsedto.ProfileLinkResponse "授予成功"
// @Failure 400 {object} core.ErrResponse "参数错误"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 409 {object} core.ErrResponse "档案关系已存在"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /identity/profile-links [post]
// @Security BearerAuth
func (h *ProfileLinkHandler) Grant(c *gin.Context) {
	var req requestdto.ProfileLinkCreateRequest
	if err := h.BindJSON(c, &req); err != nil {
		h.Error(c, err)
		return
	}

	currentUserID, ok := h.GetUserID(c)
	if !ok {
		h.ErrorWithCode(c, code.ErrTokenInvalid, "user id not found in context")
		return
	}

	result, err := h.profileLinkAccess.Grant(c.Request.Context(), currentUserID, appprofilelink.CreateProfileLinkDTO{
		UserID:    req.UserID,
		ProfileID: req.ProfileID,
		Relation:  req.Relation,
	})
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Created(c, profileLinkResultToResponse(result))
}

// Revoke 撤销关系。
// @Summary 撤销档案关系
// @Description 根据关系 ID 撤销当前用户可见的档案关系
// @Tags Identity-ProfileLink
// @Accept json
// @Produce json
// @Param id path string true "关系 ID"
// @Success 200 {object} responsedto.ProfileLinkResponse "撤销成功"
// @Failure 400 {object} core.ErrResponse "参数错误"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /identity/profile-links/{id}/revoke [post]
// @Security BearerAuth
func (h *ProfileLinkHandler) Revoke(c *gin.Context) {
	profileLinkID := c.Param("id")
	if profileLinkID == "" {
		h.ErrorWithCode(c, code.ErrInvalidArgument, "profile link id is required")
		return
	}
	currentUserID, ok := h.GetUserID(c)
	if !ok {
		h.ErrorWithCode(c, code.ErrTokenInvalid, "user id not found in context")
		return
	}
	result, err := h.profileLinkAccess.Revoke(c.Request.Context(), currentUserID, appprofilelink.RevokeProfileLinkBySelectorDTO{
		ProfileLinkID: profileLinkID,
	})
	if err != nil {
		h.Error(c, err)
		return
	}
	h.Success(c, profileLinkResultToResponse(result))
}

// List 查询档案关系
// @Summary 查询档案关系
// @Description 查询用户或档案的档案关系列表
// @Tags Identity-ProfileLink
// @Accept json
// @Produce json
// @Param user_id query string false "用户 ID"
// @Param profile_id query string false "档案 ID"
// @Param include_revoked query boolean false "是否包含已撤销档案关系"
// @Param active query boolean false "是否仅查询活跃的档案关系（兼容字段，建议使用 include_revoked）"
// @Param offset query int false "偏移量" default(0)
// @Param limit query int false "每页数量" default(20)
// @Success 200 {object} responsedto.ProfileLinkPageResponse "查询成功"
// @Failure 400 {object} core.ErrResponse "参数错误"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /identity/profile-links [get]
// @Security BearerAuth
func (h *ProfileLinkHandler) List(c *gin.Context) {
	var req requestdto.ProfileLinkListQuery
	if err := h.BindQuery(c, &req); err != nil {
		h.Error(c, err)
		return
	}

	currentUserID, ok := h.GetUserID(c)
	if !ok {
		h.ErrorWithCode(c, code.ErrTokenInvalid, "user id not found in context")
		return
	}

	results, err := h.profileLinkAccess.List(c.Request.Context(), currentUserID, appprofilelink.ListProfileLinksDTO{
		UserID:    req.UserID,
		ProfileID: req.ProfileID,
		Active:    profileLinkActiveFilter(req),
	})
	if err != nil {
		h.Error(c, err)
		return
	}

	total := len(results)
	items := make([]responsedto.ProfileLinkResponse, 0, total)
	for _, g := range results {
		if g == nil {
			continue
		}
		items = append(items, profileLinkResultToResponse(g))
	}

	sliced := sliceProfileLinks(items, req.Offset, req.Limit)

	h.Success(c, responsedto.ProfileLinkPageResponse{
		Total:  total,
		Limit:  req.Limit,
		Offset: req.Offset,
		Items:  sliced,
	})
}

func profileLinkActiveFilter(req requestdto.ProfileLinkListQuery) *bool {
	if req.IncludeRevoked != nil {
		active := !*req.IncludeRevoked
		return &active
	}
	return req.Active
}

// ========== 辅助函数 ==========

// profileLinkResultToResponse 将应用服务返回的 ProfileLinkResult 转换为响应 DTO
func profileLinkResultToResponse(result *appprofilelink.ProfileLinkResult) responsedto.ProfileLinkResponse {
	if result == nil {
		return responsedto.ProfileLinkResponse{}
	}

	resp := responsedto.ProfileLinkResponse{
		ID:        result.ID,
		UserID:    result.UserID,
		ProfileID: result.ProfileID,
		Relation:  result.Relation,
		Since:     parseProfileLinkTime(result.EstablishedAt),
	}
	if revokedAt := parseOptionalTime(result.RevokedAt); revokedAt != nil {
		resp.RevokedAt = revokedAt
	}

	return resp
}

// parseProfileLinkTime 解析时间字符串
func parseProfileLinkTime(timeStr string) time.Time {
	if timeStr == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Time{}
	}
	return t
}

// sliceProfileLinks 分页切片
func sliceProfileLinks(items []responsedto.ProfileLinkResponse, offset, limit int) []responsedto.ProfileLinkResponse {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []responsedto.ProfileLinkResponse{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}
