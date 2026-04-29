package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	requestdto "github.com/FangcunMount/iam/internal/apiserver/transport/rest/identity/request"
	responsedto "github.com/FangcunMount/iam/internal/apiserver/transport/rest/identity/response"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

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

	childResults, err := h.childAccess.ListForGuardian(c.Request.Context(), rawID)
	if err != nil {
		h.Error(c, err)
		return
	}

	var childResponses []responsedto.ChildResponse
	for _, child := range childResults {
		if child == nil {
			continue
		}
		resp := childResultToResponse(child)
		childResponses = append(childResponses, resp)
	}

	total := len(childResponses)
	sliced := sliceChildren(childResponses, query.Offset, query.Limit)

	h.Success(c, responsedto.ChildPageResponse{
		Total:  total,
		Limit:  query.Limit,
		Offset: query.Offset,
		Items:  sliced,
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

	rawUserID, ok := h.GetUserID(c)
	if !ok {
		h.ErrorWithCode(c, code.ErrTokenInvalid, "user id not found in context")
		return
	}

	child, err := h.childAccess.GetForGuardian(c.Request.Context(), rawUserID, childID)
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
