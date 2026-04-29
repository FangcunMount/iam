package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	appprofile "github.com/FangcunMount/iam/internal/apiserver/application/uc/profile"
	requestdto "github.com/FangcunMount/iam/internal/apiserver/transport/rest/identity/request"
	responsedto "github.com/FangcunMount/iam/internal/apiserver/transport/rest/identity/response"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/pkg/core"
)

var _ = core.ErrResponse{}

// CreateProfile 创建档案并建立当前用户关系。
// @Summary 创建档案并建立关系
// @Description 创建档案并自动将当前用户关联到该档案（身份证可不填写）
// @Tags Identity-Profiles
// @Accept json
// @Produce json
// @Param request body requestdto.ProfileCreateRequest true "创建档案请求"
// @Success 201 {object} responsedto.ProfileCreateResponse "创建成功"
// @Failure 400 {object} core.ErrResponse "请求参数错误"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 409 {object} core.ErrResponse "档案已存在"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /identity/profiles [post]
// @Security BearerAuth
func (h *ProfileHandler) CreateProfile(c *gin.Context) {
	var req requestdto.ProfileCreateRequest
	if err := h.BindJSON(c, &req); err != nil {
		h.Error(c, err)
		return
	}

	rawUserID, ok := h.GetUserID(c)
	if !ok {
		h.ErrorWithCode(c, code.ErrTokenInvalid, "user id not found in context")
		return
	}

	gender := uint8(0)
	if req.Gender != nil {
		gender = *req.Gender
	}
	result, err := h.myProfiles.Create(c.Request.Context(), rawUserID, appprofile.CreateMyProfileDTO{
		Name:     strings.TrimSpace(req.LegalName),
		Gender:   gender,
		Birthday: strings.TrimSpace(req.DOB),
		IDCard:   strings.TrimSpace(req.IDNo),
		Height:   parseHeightCm(req.HeightCm),
		Weight:   parseWeightKg(req.WeightKg),
		Relation: req.Relation,
	})
	if err != nil {
		h.Error(c, err)
		return
	}
	profileResult := result.Profile
	profileLinkResult := result.ProfileLink

	profileResp := responsedto.ProfileResponse{
		ID:        profileResult.ID,
		LegalName: profileResult.Name,
		Gender:    &profileResult.Gender,
		DOB:       profileResult.Birthday,
		IDType:    req.IDType,
		IDMasked:  maskIDCard(profileResult.IDCard),
	}

	profileLinkResp := responsedto.ProfileLinkResponse{
		ID:        profileLinkResult.ID,
		UserID:    profileLinkResult.UserID,
		ProfileID: profileLinkResult.ProfileID,
		Relation:  profileLinkResult.Relation,
		Since:     parseTime(profileLinkResult.EstablishedAt),
	}
	if revokedAt := parseOptionalTime(profileLinkResult.RevokedAt); revokedAt != nil {
		profileLinkResp.RevokedAt = revokedAt
	}

	h.Created(c, responsedto.ProfileCreateResponse{
		Profile:     profileResp,
		ProfileLink: profileLinkResp,
	})
}

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
// @Router /identity/profiles/{id} [patch]
// @Security BearerAuth
func (h *ProfileHandler) PatchProfile(c *gin.Context) {
	profileID := c.Param("id")
	if strings.TrimSpace(profileID) == "" {
		h.ErrorWithCode(c, code.ErrInvalidArgument, "profile id is required")
		return
	}

	rawUserID, ok := h.GetUserID(c)
	if !ok {
		h.ErrorWithCode(c, code.ErrTokenInvalid, "user id not found in context")
		return
	}

	var req requestdto.ProfileUpdateRequest
	if err := h.BindJSON(c, &req); err != nil {
		h.Error(c, err)
		return
	}

	var height *uint32
	if req.HeightCm != nil {
		parsed := uint32(*req.HeightCm)
		height = &parsed
	}
	var weight *uint32
	if req.WeightKg != nil {
		f, _ := strconv.ParseFloat(strings.TrimSpace(*req.WeightKg), 64)
		parsed := uint32(f * 1000)
		weight = &parsed
	}

	profile, err := h.myProfiles.Patch(c.Request.Context(), appprofile.PatchMyProfileDTO{
		UserID:    rawUserID,
		ProfileID: profileID,
		LegalName: req.LegalName,
		Gender:    req.Gender,
		Birthday:  req.DOB,
		Height:    height,
		Weight:    weight,
	})
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, profileResultToResponse(profile))
}
