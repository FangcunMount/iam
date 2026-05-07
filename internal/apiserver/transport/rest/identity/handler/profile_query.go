package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	requestdto "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/identity/request"
	responsedto "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/identity/response"
	"github.com/FangcunMount/iam/v2/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v2/pkg/core"
)

var _ = core.ErrResponse{}

// ListMyProfiles 获取当前用户的档案列表
// @Summary 获取当前用户的档案列表
// @Description 获取当前登录用户作为关系用户的所有档案
// @Tags Identity-Profiles
// @Accept json
// @Produce json
// @Param offset query int false "偏移量" default(0)
// @Param limit query int false "每页数量" default(20)
// @Success 200 {object} responsedto.ProfilePageResponse "查询成功"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /identity/me/profiles [get]
// @Security BearerAuth
func (h *ProfileHandler) ListMyProfiles(c *gin.Context) {
	var query requestdto.ProfileListQuery
	if err := h.BindQuery(c, &query); err != nil {
		h.Error(c, err)
		return
	}

	userID, err := requestctx.RequiredUserID(c)
	if err != nil {
		h.Error(c, err)
		return
	}

	profileResults, err := h.myProfiles.List(c.Request.Context(), userID)
	if err != nil {
		h.Error(c, err)
		return
	}

	var profileResponses []responsedto.ProfileResponse
	for _, profile := range profileResults {
		if profile == nil {
			continue
		}
		resp := profileResultToResponse(profile)
		profileResponses = append(profileResponses, resp)
	}

	total := len(profileResponses)
	sliced := sliceProfiles(profileResponses, query.Offset, query.Limit)

	h.Success(c, responsedto.ProfilePageResponse{
		Total:  total,
		Limit:  query.Limit,
		Offset: query.Offset,
		Items:  sliced,
	})
}

// GetProfile 查询档案
// @Summary 查询档案（仅限当前用户可访问的档案）
// @Description 根据档案 ID 查询档案详情，只能查询当前用户关系的档案
// @Tags Identity-Profiles
// @Accept json
// @Produce json
// @Param id path string true "档案 ID"
// @Success 200 {object} responsedto.ProfileResponse "查询成功"
// @Failure 400 {object} core.ErrResponse "参数错误"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 403 {object} core.ErrResponse "无权限访问此档案"
// @Failure 404 {object} core.ErrResponse "档案不存在"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /identity/profiles/{id} [get]
// @Security BearerAuth
func (h *ProfileHandler) GetProfile(c *gin.Context) {
	profileID, err := parseProfileID(c.Param("id"))
	if err != nil {
		h.Error(c, err)
		return
	}

	userID, err := requestctx.RequiredUserID(c)
	if err != nil {
		h.Error(c, err)
		return
	}

	profile, err := h.myProfiles.Get(c.Request.Context(), userID, profileID)
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, profileResultToResponse(profile))
}

// SearchProfiles 搜索相似档案（根据姓名、性别、生日）
// @Summary 搜索档案
// @Description 根据姓名、生日等信息搜索相似的档案（用于运营查询）
// @Tags Identity-Profiles
// @Accept json
// @Produce json
// @Param name query string false "档案姓名"
// @Param dob query string false "出生日期 YYYY-MM-DD"
// @Param offset query int false "偏移量" default(0)
// @Param limit query int false "每页数量" default(20)
// @Success 200 {object} responsedto.ProfilePageResponse "查询成功"
// @Failure 400 {object} core.ErrResponse "参数错误"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /identity/profiles/search [get]
// @Security BearerAuth
func (h *ProfileHandler) SearchProfiles(c *gin.Context) {
	var query requestdto.ProfileSearchQuery
	if err := h.BindQuery(c, &query); err != nil {
		h.Error(c, err)
		return
	}

	name := strings.TrimSpace(query.Name)
	birthday := ""
	if query.DOB != nil {
		birthday = strings.TrimSpace(*query.DOB)
	}

	profiles, err := h.profileDirectory.FindSimilar(c.Request.Context(), name, 0, birthday)
	if err != nil {
		h.Error(c, err)
		return
	}

	var items []responsedto.ProfileResponse
	for _, profile := range profiles {
		if profile != nil {
			items = append(items, profileResultToResponse(profile))
		}
	}

	total := len(items)
	sliced := sliceProfiles(items, query.Offset, query.Limit)

	h.Success(c, responsedto.ProfilePageResponse{
		Total:  total,
		Limit:  query.Limit,
		Offset: query.Offset,
		Items:  sliced,
	})
}
