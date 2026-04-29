package handler

import (
	"github.com/gin-gonic/gin"

	resp "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/response"
)

var _ = resp.Account{}

// GetAccountByID 根据账户ID获取账户信息
// @Summary 获取账户信息
// @Description 根据账户ID获取账户详细信息
// @Tags 账户管理
// @Accept json
// @Produce json
// @Param accountId path string true "账户ID"
// @Success 200 {object} resp.Account "账户信息"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 404 {object} map[string]interface{} "账户不存在"
// @Router /authn/accounts/{accountId} [get]
func (h *AccountHandler) GetAccountByID(c *gin.Context) {
	accountID, err := parseAccountID(c.Param("accountId"))
	if err != nil {
		h.Error(c, err)
		return
	}

	result, err := h.accountService.GetAccountByID(c.Request.Context(), accountID)
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, toAccountResponse(result))
}
