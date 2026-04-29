package handler

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	appAccount "github.com/FangcunMount/iam/internal/apiserver/application/authn/account"
	appRegister "github.com/FangcunMount/iam/internal/apiserver/application/authn/register"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	req "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/request"
	resp "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/response"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	_ "github.com/FangcunMount/iam/pkg/core"
)

// AccountHandler 账户管理 HTTP Handler
type AccountHandler struct {
	*BaseHandler
	accountService  appAccount.AccountApplicationService
	registerService appRegister.RegisterApplicationService
}

// NewAccountHandler 创建账户处理器
func NewAccountHandler(
	accountService appAccount.AccountApplicationService,
	registerService appRegister.RegisterApplicationService,
) *AccountHandler {
	return &AccountHandler{
		BaseHandler:     NewBaseHandler(),
		accountService:  accountService,
		registerService: registerService,
	}
}

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

// RegisterWithWeChat 微信用户注册
// @Summary 微信用户注册
// @Description 使用微信 JS Code 注册新用户，服务端根据 AppID 查询 AppSecret 并自动调用 code2session 获取 OpenID 和 UnionID
// @Tags 账户管理
// @Accept json
// @Produce json
// @Param request body req.RegisterWeChatAccountReq true "微信注册请求"
// @Success 201 {object} resp.RegisterResult "注册成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 409 {object} map[string]interface{} "用户已存在"
// @Router /authn/accounts/wechat/register [post]
func (h *AccountHandler) RegisterWithWeChat(c *gin.Context) {
	var reqBody req.RegisterWeChatAccountReq
	if err := h.BindJSON(c, &reqBody); err != nil {
		h.Error(c, err)
		return
	}

	var phone meta.Phone
	if strings.TrimSpace(reqBody.Phone) != "" {
		phone, _ = meta.NewPhone(strings.TrimSpace(reqBody.Phone))
	}
	var email meta.Email
	if strings.TrimSpace(reqBody.Email) != "" {
		email, _ = meta.NewEmail(strings.TrimSpace(reqBody.Email))
	}

	appID := strings.TrimSpace(reqBody.AppID)
	jsCode := strings.TrimSpace(reqBody.JsCode)

	profile := make(map[string]string)
	if reqBody.Nickname != nil && *reqBody.Nickname != "" {
		profile["nickname"] = *reqBody.Nickname
	}
	if reqBody.Avatar != nil && *reqBody.Avatar != "" {
		profile["avatar"] = *reqBody.Avatar
	}

	metaMap, err := reqBody.MetaJSON()
	if err != nil {
		h.Error(c, err)
		return
	}

	registerReq := appRegister.RegisterRequest{
		Name:           strings.TrimSpace(reqBody.Name),
		Phone:          phone,
		Email:          email,
		AccountType:    domain.TypeWcMinip,
		CredentialType: appRegister.CredTypeWechat,
		WechatAppID:    &appID,
		WechatJsCode:   &jsCode,
		Profile:        profile,
		Meta:           metaMap,
	}

	result, err := h.registerService.Register(c.Request.Context(), registerReq)
	if err != nil {
		h.Error(c, err)
		return
	}

	response := resp.RegisterResult{
		UserID:       result.UserID.Uint64(),
		UserName:     result.UserName,
		Phone:        result.Phone.String(),
		Email:        result.Email.String(),
		AccountID:    result.AccountID.Uint64(),
		AccountType:  string(result.AccountType),
		ExternalID:   string(result.ExternalID),
		CredentialID: result.CredentialID.Uint64(),
		IsNewUser:    result.IsNewUser,
		IsNewAccount: result.IsNewAccount,
	}

	h.Success(c, response)
}

// EnsureMockConsumer 内部 mock C 端账户 ensure。
func (h *AccountHandler) EnsureMockConsumer(c *gin.Context) {
	var reqBody req.EnsureMockConsumerReq
	if err := h.BindJSON(c, &reqBody); err != nil {
		h.Error(c, err)
		return
	}
	if err := reqBody.Validate(); err != nil {
		h.Error(c, err)
		return
	}

	phone, err := meta.NewPhone(strings.TrimSpace(reqBody.Phone))
	if err != nil {
		h.Error(c, perrors.WithCode(code.ErrInvalidArgument, "invalid phone: %v", err))
		return
	}
	email, err := meta.NewEmail(strings.TrimSpace(reqBody.Email))
	if err != nil {
		h.Error(c, perrors.WithCode(code.ErrInvalidArgument, "invalid email: %v", err))
		return
	}

	password := strings.TrimSpace(reqBody.Password)
	registerReq := appRegister.RegisterRequest{
		Name:           strings.TrimSpace(reqBody.Name),
		Phone:          phone,
		Email:          email,
		AccountType:    domain.TypeMockConsumer,
		CredentialType: appRegister.CredTypePassword,
		Password:       &password,
		Profile:        reqBody.Profile,
		Meta:           reqBody.Meta,
	}

	result, err := h.registerService.Register(c.Request.Context(), registerReq)
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, resp.EnsureMockConsumerResult{
		UserID:       result.UserID.String(),
		AccountID:    result.AccountID.String(),
		LoginID:      strings.TrimSpace(reqBody.Email),
		IsNewUser:    result.IsNewUser,
		IsNewAccount: result.IsNewAccount,
	})
}

// parseAccountID 解析账户ID
func parseAccountID(idStr string) (meta.ID, error) {
	idStr = strings.TrimSpace(idStr)
	if idStr == "" {
		return meta.FromUint64(0), perrors.WithCode(code.ErrInvalidArgument, "accountId is required")
	}

	var id uint64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		return meta.FromUint64(0), perrors.WithCode(code.ErrInvalidArgument, "invalid accountId format")
	}

	return meta.FromUint64(id), nil
}

// toAccountResponse 转换为账户响应（简化版）
func toAccountResponse(result *appAccount.AccountResult) resp.Account {
	appIDStr := string(result.AppID)
	return resp.Account{
		ID:         result.AccountID.String(),
		UserID:     result.UserID.String(),
		Provider:   string(result.Type),
		ExternalID: string(result.ExternalID),
		AppID:      &appIDStr,
		Status:     result.Status.String(),
	}
}

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

	profile := make(map[string]string)
	if reqBody.Nickname != nil && *reqBody.Nickname != "" {
		profile["nickname"] = *reqBody.Nickname
	}
	if reqBody.Avatar != nil && *reqBody.Avatar != "" {
		profile["avatar"] = *reqBody.Avatar
	}

	// 将 Meta 转换为 map[string]string
	if reqBody.Meta != nil {
		for k, v := range reqBody.Meta {
			if str, ok := v.(string); ok {
				profile[k] = str
			}
		}
	}

	if err := h.accountService.UpdateProfile(c.Request.Context(), accountID, profile); err != nil {
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
	if err := h.accountService.SetUniqueID(c.Request.Context(), accountID, unionID); err != nil {
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

	if err := h.accountService.DisableAccount(c.Request.Context(), accountID); err != nil {
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

	if err := h.accountService.EnableAccount(c.Request.Context(), accountID); err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, resp.MessageResponse{Message: "Account enabled successfully"})
}
