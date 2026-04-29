package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	appchild "github.com/FangcunMount/iam/internal/apiserver/application/uc/child"
	appregistration "github.com/FangcunMount/iam/internal/apiserver/application/uc/registration"
	requestdto "github.com/FangcunMount/iam/internal/apiserver/transport/rest/identity/request"
	responsedto "github.com/FangcunMount/iam/internal/apiserver/transport/rest/identity/response"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// RegisterChild 注册儿童并授予当前用户监护权
// @Summary 注册儿童档案并建立监护关系
// @Description 创建儿童档案并自动将当前用户设置为监护人（身份证可不填写）
// @Tags Identity-Children
// @Accept json
// @Produce json
// @Param request body requestdto.ChildRegisterRequest true "注册儿童请求"
// @Success 201 {object} responsedto.ChildRegisterResponse "注册成功"
// @Failure 400 {object} core.ErrResponse "请求参数错误"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 409 {object} core.ErrResponse "儿童已存在"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /identity/children/register [post]
// @Security BearerAuth
func (h *ChildHandler) RegisterChild(c *gin.Context) {
	var req requestdto.ChildRegisterRequest
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
	result, err := h.registrationApp.RegisterChildWithGuardian(c.Request.Context(), appregistration.RegisterChildWithGuardianDTO{
		UserID:   rawUserID,
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
	childResult := result.Child
	guardResult := result.Guardianship

	childResp := responsedto.ChildResponse{
		ID:        childResult.ID,
		LegalName: childResult.Name,
		Gender:    &childResult.Gender,
		DOB:       childResult.Birthday,
		IDType:    req.IDType,
		IDMasked:  maskIDCard(childResult.IDCard),
	}

	guardResp := responsedto.GuardianshipResponse{
		ID:       guardResult.ID,
		UserID:   guardResult.UserID,
		ChildID:  guardResult.ChildID,
		Relation: guardResult.Relation,
		Since:    parseTime(guardResult.EstablishedAt),
	}
	if revokedAt := parseOptionalTime(guardResult.RevokedAt); revokedAt != nil {
		guardResp.RevokedAt = revokedAt
	}

	h.Created(c, responsedto.ChildRegisterResponse{
		Child:        childResp,
		Guardianship: guardResp,
	})
}

// PatchChild 更新儿童档案
// @Summary 更新儿童档案（仅限自己监护的孩子）
// @Description 部分更新儿童档案信息，只能更新当前用户监护的孩子
// @Tags Identity-Children
// @Accept json
// @Produce json
// @Param id path string true "儿童 ID"
// @Param request body requestdto.ChildUpdateRequest true "更新儿童请求"
// @Success 200 {object} responsedto.ChildResponse "更新成功"
// @Failure 400 {object} core.ErrResponse "参数错误"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 403 {object} core.ErrResponse "无权限修改此儿童"
// @Failure 404 {object} core.ErrResponse "儿童不存在"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /identity/children/{id} [patch]
// @Security BearerAuth
func (h *ChildHandler) PatchChild(c *gin.Context) {
	childID := c.Param("id")
	if strings.TrimSpace(childID) == "" {
		h.ErrorWithCode(c, code.ErrInvalidArgument, "child id is required")
		return
	}

	rawUserID, ok := h.GetUserID(c)
	if !ok {
		h.ErrorWithCode(c, code.ErrTokenInvalid, "user id not found in context")
		return
	}

	var req requestdto.ChildUpdateRequest
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

	child, err := h.childAccess.PatchForGuardian(c.Request.Context(), appchild.PatchChildForGuardianDTO{
		UserID:    rawUserID,
		ChildID:   childID,
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

	h.Success(c, childResultToResponse(child))
}
