package handler

import (
	"github.com/gin-gonic/gin"

	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	req "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/request"
	resp "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/response"
)

// UpdateProfile 更新账户资料
// @Summary 更新账户资料
// @Description 更新微信账户的昵称、头像等资料信息
// @Tags 账户管理
// @Accept json
// @Produce json
// @Param accountId path string true "账户ID"
// @Param request body req.UpsertWeChatProfileReq true "更新资料请求"
// @Success 200 {object} resp.MessageResponse "更新成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 404 {object} map[string]interface{} "账户不存在"
// @Router /authn/accounts/{accountId}/profile [put]
func (h *AccountHandler) UpdateProfile(c *gin.Context) {
	accountID, err := parseAccountID(c.Param("accountId"))
	if err != nil {
		h.Error(c, err)
		return
	}

	var reqBody req.UpsertWeChatProfileReq
	if err := h.BindJSON(c, &reqBody); err != nil {
		h.Error(c, err)
		return
	}

	if err := reqBody.Validate(); err != nil {
		h.Error(c, err)
		return
	}

	if err := h.profileEditor.UpdateProfile(c.Request.Context(), accountID, profileFromUpsertRequest(reqBody)); err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, resp.MessageResponse{Message: "Profile updated successfully"})
}

// SetUnionID 设置账户的 UnionID
// @Summary 设置账户 UnionID
// @Description 将微信账户的 UnionID 与内部账户关联
// @Tags 账户管理
// @Accept json
// @Produce json
// @Param accountId path string true "账户ID"
// @Param request body req.SetWeChatUnionIDReq true "设置 UnionID 请求"
// @Success 200 {object} resp.MessageResponse "设置成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 404 {object} map[string]interface{} "账户不存在"
// @Router /authn/accounts/{accountId}/unionid [put]
func (h *AccountHandler) SetUnionID(c *gin.Context) {
	accountID, err := parseAccountID(c.Param("accountId"))
	if err != nil {
		h.Error(c, err)
		return
	}

	var reqBody req.SetWeChatUnionIDReq
	if err := h.BindJSON(c, &reqBody); err != nil {
		h.Error(c, err)
		return
	}

	if err := reqBody.Validate(); err != nil {
		h.Error(c, err)
		return
	}

	unionID := domain.UnionID(reqBody.UnionID)
	if err := h.profileEditor.SetUniqueID(c.Request.Context(), accountID, unionID); err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, resp.MessageResponse{Message: "UnionID set successfully"})
}

// DisableAccount 禁用账户
// @Summary 禁用账户
// @Description 将账户标记为禁用，阻止继续认证
// @Tags 账户管理
// @Param accountId path string true "账户ID"
// @Success 200 {object} resp.MessageResponse "禁用成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 404 {object} map[string]interface{} "账户不存在"
// @Router /authn/accounts/{accountId}/disable [post]
func (h *AccountHandler) DisableAccount(c *gin.Context) {
	accountID, err := parseAccountID(c.Param("accountId"))
	if err != nil {
		h.Error(c, err)
		return
	}

	if err := h.statusChanger.DisableAccount(c.Request.Context(), accountID); err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, resp.MessageResponse{Message: "Account disabled successfully"})
}

// EnableAccount 启用账户
// @Summary 启用账户
// @Description 恢复已禁用的账户
// @Tags 账户管理
// @Param accountId path string true "账户ID"
// @Success 200 {object} resp.MessageResponse "启用成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 404 {object} map[string]interface{} "账户不存在"
// @Router /authn/accounts/{accountId}/enable [post]
func (h *AccountHandler) EnableAccount(c *gin.Context) {
	accountID, err := parseAccountID(c.Param("accountId"))
	if err != nil {
		h.Error(c, err)
		return
	}

	if err := h.statusChanger.EnableAccount(c.Request.Context(), accountID); err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, resp.MessageResponse{Message: "Account enabled successfully"})
}
