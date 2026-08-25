package handler

import (
	"github.com/gin-gonic/gin"

	appprofile "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/profile"
	requestdto "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/identity/request"
	responsedto "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/identity/response"
	"github.com/FangcunMount/iam/v3/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v3/pkg/core"
)

var _ = core.ErrResponse{}
var _ = responsedto.ProfileResponse{}

// PatchProfile 更新档案。
// @Summary 更新档案
// @Description 部分更新当前用户可访问的档案信息
// @Tags Identity-Profiles
// @Accept json
// @Produce json
// @Param id path string true "档案 ID"
// @Param request body requestdto.ProfileUpdateRequest true "更新档案请求"
// @Success 200 {object} responsedto.ProfileResponse "更新成功"
// @Failure 400 {object} core.ErrResponse "参数错误"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 403 {object} core.ErrResponse "无权限修改此档案"
// @Failure 404 {object} core.ErrResponse "档案不存在"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /v2/identity/profiles/{id} [patch]
// @Security BearerAuth
func (h *ProfileHandler) PatchProfile(c *gin.Context) {
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

	var req requestdto.ProfileUpdateRequest
	if err := h.BindJSON(c, &req); err != nil {
		h.Error(c, err)
		return
	}

	profile, err := h.myProfiles.Patch(c.Request.Context(), appprofile.PatchMyProfileDTO{
		UserID:    userID,
		ProfileID: profileID,
		LegalName: req.LegalName,
		Gender:    req.Gender,
		Birthday:  req.DOB,
	})
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, profileResultToResponse(profile))
}
