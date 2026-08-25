package handler

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/gin-gonic/gin"

	appprofilelink "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/profilelink"
	requestdto "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/identity/request"
	responsedto "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/identity/response"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/FangcunMount/iam/v3/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v3/pkg/core"
)

var _ = core.ErrResponse{}

// ProfileLinkHandler 档案关系 REST 处理器
type ProfileLinkHandler struct {
	*BaseHandler
	profileLinkAccess profileLinkLister
}

type profileLinkLister interface {
	List(ctx context.Context, currentUserID meta.ID, dto appprofilelink.ListProfileLinksDTO) ([]*appprofilelink.ProfileLinkResult, error)
}

// NewProfileLinkHandler 创建关系处理器
func NewProfileLinkHandler(
	profileLinkAccess profileLinkLister,
) *ProfileLinkHandler {
	return &ProfileLinkHandler{
		BaseHandler:       NewBaseHandler(),
		profileLinkAccess: profileLinkAccess,
	}
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
// @Param offset query int false "偏移量" default(0)
// @Param limit query int false "每页数量" default(20)
// @Success 200 {object} responsedto.ProfileLinkPageResponse "查询成功"
// @Failure 400 {object} core.ErrResponse "参数错误"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /v2/identity/profile-links [get]
// @Security BearerAuth
func (h *ProfileLinkHandler) List(c *gin.Context) {
	if c.Request.URL.Query().Has("active") {
		h.Error(c, perrors.WithCode(code.ErrInvalidArgument, "query parameter active has been removed; use include_revoked"))
		return
	}

	var req requestdto.ProfileLinkListQuery
	if err := h.BindQuery(c, &req); err != nil {
		h.Error(c, err)
		return
	}
	currentUserID, err := requestctx.RequiredUserID(c)
	if err != nil {
		h.Error(c, err)
		return
	}
	userID, err := parseOptionalID(req.UserID, "user id")
	if err != nil {
		h.Error(c, err)
		return
	}
	profileID, err := parseOptionalID(req.ProfileID, "profile id")
	if err != nil {
		h.Error(c, err)
		return
	}

	results, err := h.profileLinkAccess.List(c.Request.Context(), currentUserID, appprofilelink.ListProfileLinksDTO{
		UserID:         userID,
		ProfileID:      profileID,
		IncludeRevoked: req.IncludeRevoked != nil && *req.IncludeRevoked,
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
