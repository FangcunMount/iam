package handler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	appchild "github.com/FangcunMount/iam/internal/apiserver/application/uc/child"
	appguard "github.com/FangcunMount/iam/internal/apiserver/application/uc/guardianship"
	appregistration "github.com/FangcunMount/iam/internal/apiserver/application/uc/registration"
	requestdto "github.com/FangcunMount/iam/internal/apiserver/interface/uc/restful/request"
	responsedto "github.com/FangcunMount/iam/internal/apiserver/interface/uc/restful/response"
	"github.com/FangcunMount/iam/internal/pkg/code"
	_ "github.com/FangcunMount/iam/pkg/core" // imported for swagger
)

// ChildHandler 儿童档案 REST 处理器
type ChildHandler struct {
	*BaseHandler
	childApp        appchild.ChildApplicationService
	profileApp      appchild.ChildProfileApplicationService
	registrationApp appregistration.ChildRegistrationService
	guardQuery      appguard.GuardianshipQueryApplicationService
	childQuery      appchild.ChildQueryApplicationService
}

// NewChildHandler 创建儿童档案处理器
func NewChildHandler(
	childApp appchild.ChildApplicationService,
	profileApp appchild.ChildProfileApplicationService,
	registrationApp appregistration.ChildRegistrationService,
	guardQuery appguard.GuardianshipQueryApplicationService,
	childQuery appchild.ChildQueryApplicationService,
) *ChildHandler {
	return &ChildHandler{
		BaseHandler:     NewBaseHandler(),
		childApp:        childApp,
		profileApp:      profileApp,
		registrationApp: registrationApp,
		guardQuery:      guardQuery,
		childQuery:      childQuery,
	}
}

// ListMyChildren 获取当前用户的儿童档案列表
// @Summary 获取当前用户的儿童档案列表
// @Description 获取当前登录用户作为监护人的所有儿童档案
// @Tags Identity-Children
// @Accept json
// @Produce json
// @Param offset query int false "偏移量" default(0)
// @Param limit query int false "每页数量" default(20)
// @Success 200 {object} responsedto.ChildPageResponse "查询成功"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /identity/me/children [get]
// @Security BearerAuth
func (h *ChildHandler) ListMyChildren(c *gin.Context) {
	var query requestdto.ChildListQuery
	if err := h.BindQuery(c, &query); err != nil {
		h.Error(c, err)
		return
	}

	rawID, ok := h.GetUserID(c)
	if !ok {
		h.ErrorWithCode(c, code.ErrTokenInvalid, "user id not found in context")
		return
	}

	// 列出用户监护的所有儿童
	guardianships, err := h.guardQuery.ListChildrenByUserID(c.Request.Context(), rawID)
	if err != nil {
		h.Error(c, err)
		return
	}

	var children []responsedto.ChildResponse
	for _, g := range guardianships {
		if g == nil {
			continue
		}
		// 查询儿童详细信息
		child, err := h.childQuery.GetByID(c.Request.Context(), g.ChildID)
		if err != nil {
			h.Error(c, err)
			return
		}
		resp := childResultToResponse(child)
		children = append(children, resp)
	}

	total := len(children)
	sliced := sliceChildren(children, query.Offset, query.Limit)

	h.Success(c, responsedto.ChildPageResponse{
		Total:  total,
		Limit:  query.Limit,
		Offset: query.Offset,
		Items:  sliced,
	})
}

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

	// 构建响应
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

// GetChild 查询儿童档案
// @Summary 查询儿童档案（仅限自己监护的孩子）
// @Description 根据儿童 ID 查询儿童详细档案，只能查询当前用户监护的孩子
// @Tags Identity-Children
// @Accept json
// @Produce json
// @Param id path string true "儿童 ID"
// @Success 200 {object} responsedto.ChildResponse "查询成功"
// @Failure 400 {object} core.ErrResponse "参数错误"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 403 {object} core.ErrResponse "无权限访问此儿童"
// @Failure 404 {object} core.ErrResponse "儿童不存在"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /identity/children/{id} [get]
// @Security BearerAuth
func (h *ChildHandler) GetChild(c *gin.Context) {
	childID := c.Param("id")
	if strings.TrimSpace(childID) == "" {
		h.ErrorWithCode(c, code.ErrInvalidArgument, "child id is required")
		return
	}

	// 获取当前用户ID
	rawUserID, ok := h.GetUserID(c)
	if !ok {
		h.ErrorWithCode(c, code.ErrTokenInvalid, "user id not found in context")
		return
	}

	ctx := c.Request.Context()

	// 验证用户是否是该儿童的监护人
	guardianship, err := h.guardQuery.GetByUserIDAndChildID(ctx, rawUserID, childID)
	if err != nil {
		h.ErrorWithCode(c, code.ErrPermissionDenied, "you are not the guardian of this child")
		return
	}

	// 检查监护关系是否有效
	if guardianship == nil {
		h.ErrorWithCode(c, code.ErrPermissionDenied, "you are not the guardian of this child")
		return
	}

	// 查询儿童信息
	child, err := h.childQuery.GetByID(ctx, childID)
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, childResultToResponse(child))
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

	// 获取当前用户ID
	rawUserID, ok := h.GetUserID(c)
	if !ok {
		h.ErrorWithCode(c, code.ErrTokenInvalid, "user id not found in context")
		return
	}

	ctx := c.Request.Context()

	// 验证用户是否是该儿童的监护人
	guardianship, err := h.guardQuery.GetByUserIDAndChildID(ctx, rawUserID, childID)
	if err != nil {
		h.ErrorWithCode(c, code.ErrPermissionDenied, "you are not the guardian of this child")
		return
	}

	// 检查监护关系是否有效
	if guardianship == nil {
		h.ErrorWithCode(c, code.ErrPermissionDenied, "you are not the guardian of this child")
		return
	}

	var req requestdto.ChildUpdateRequest
	if err := h.BindJSON(c, &req); err != nil {
		h.Error(c, err)
		return
	}

	// 更新姓名
	if req.LegalName != nil && strings.TrimSpace(*req.LegalName) != "" {
		if err := h.profileApp.Rename(ctx, childID, strings.TrimSpace(*req.LegalName)); err != nil {
			h.Error(c, err)
			return
		}
	}

	// 更新性别和生日
	if req.Gender != nil || req.DOB != nil {
		dto := appchild.UpdateChildProfileDTO{
			ChildID: childID,
		}
		if req.Gender != nil {
			dto.Gender = *req.Gender
		}
		if req.DOB != nil {
			dto.Birthday = strings.TrimSpace(*req.DOB)
		}
		if err := h.profileApp.UpdateProfile(ctx, dto); err != nil {
			h.Error(c, err)
			return
		}
	}

	// 更新身高体重
	if req.HeightCm != nil || req.WeightKg != nil {
		height := uint32(0)
		weight := uint32(0)

		if req.HeightCm != nil {
			height = uint32(*req.HeightCm)
		}
		if req.WeightKg != nil {
			f, _ := strconv.ParseFloat(strings.TrimSpace(*req.WeightKg), 64)
			weight = uint32(f * 1000) // kg转克
		}

		dto := appchild.UpdateHeightWeightDTO{
			ChildID: childID,
			Height:  height,
			Weight:  weight,
		}
		if err := h.profileApp.UpdateHeightWeight(ctx, dto); err != nil {
			h.Error(c, err)
			return
		}
	}

	// 返回更新后的儿童信息
	child, err := h.childQuery.GetByID(ctx, childID)
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, childResultToResponse(child))
}

// SearchChildren 搜索相似儿童（根据姓名、性别、生日）
// @Summary 搜索儿童
// @Description 根据姓名、生日等信息搜索相似的儿童档案（用于运营查询）
// @Tags Identity-Children
// @Accept json
// @Produce json
// @Param name query string false "儿童姓名"
// @Param dob query string false "出生日期 YYYY-MM-DD"
// @Param offset query int false "偏移量" default(0)
// @Param limit query int false "每页数量" default(20)
// @Success 200 {object} responsedto.ChildPageResponse "查询成功"
// @Failure 400 {object} core.ErrResponse "参数错误"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /identity/children/search [get]
// @Security BearerAuth
func (h *ChildHandler) SearchChildren(c *gin.Context) {
	var query requestdto.ChildSearchQuery
	if err := h.BindQuery(c, &query); err != nil {
		h.Error(c, err)
		return
	}

	name := strings.TrimSpace(query.Name)
	birthday := ""
	if query.DOB != nil {
		birthday = strings.TrimSpace(*query.DOB)
	}

	// SearchQuery 中没有 Gender，这里使用 0 作为默认值（表示不过滤）
	children, err := h.childQuery.FindSimilar(c.Request.Context(), name, 0, birthday)
	if err != nil {
		h.Error(c, err)
		return
	}

	var items []responsedto.ChildResponse
	for _, child := range children {
		if child != nil {
			items = append(items, childResultToResponse(child))
		}
	}

	total := len(items)
	sliced := sliceChildren(items, query.Offset, query.Limit)

	h.Success(c, responsedto.ChildPageResponse{
		Total:  total,
		Limit:  query.Limit,
		Offset: query.Offset,
		Items:  sliced,
	})
}

// ========== 辅助函数 ==========

// childResultToResponse 将应用服务返回的 ChildResult 转换为响应 DTO
func childResultToResponse(result *appchild.ChildResult) responsedto.ChildResponse {
	if result == nil {
		return responsedto.ChildResponse{}
	}

	resp := responsedto.ChildResponse{
		ID:        result.ID,
		LegalName: result.Name,
		DOB:       result.Birthday,
		IDMasked:  maskIDCard(result.IDCard),
	}

	// 性别转换
	resp.Gender = &result.Gender

	// 身高（厘米）
	if result.Height > 0 {
		h := int(result.Height)
		resp.HeightCm = &h
	}

	// 体重（千克，字符串格式）
	if result.Weight > 0 {
		kg := float64(result.Weight) / 1000.0
		w := fmt.Sprintf("%.2f", kg)
		resp.WeightKg = &w
	}

	return resp
}

// parseHeightCm 解析身高（厘米）
func parseHeightCm(heightCm *int) *uint32 {
	if heightCm == nil || *heightCm <= 0 {
		return nil
	}
	h := uint32(*heightCm)
	return &h
}

// parseWeightKg 解析体重（千克字符串转克）
func parseWeightKg(weightKg string) *uint32 {
	if strings.TrimSpace(weightKg) == "" {
		return nil
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(weightKg), 64)
	if err != nil || f <= 0 {
		return nil
	}
	w := uint32(f * 1000) // 千克转克
	return &w
}

// maskIDCard 脱敏身份证号
func maskIDCard(idCard string) string {
	if len(idCard) < 6 {
		return idCard
	}
	return idCard[:6] + "********" + idCard[len(idCard)-4:]
}

// sliceChildren 分页切片
func sliceChildren(items []responsedto.ChildResponse, offset, limit int) []responsedto.ChildResponse {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []responsedto.ChildResponse{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

// parseTime 解析时间字符串（ISO 8601 格式）
func parseTime(timeStr string) time.Time {
	if timeStr == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Time{}
	}
	return t
}

func parseOptionalTime(timeStr string) *time.Time {
	if timeStr == "" {
		return nil
	}
	parsed := parseTime(timeStr)
	if parsed.IsZero() {
		return nil
	}
	return &parsed
}
