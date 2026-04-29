package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	appRegister "github.com/FangcunMount/iam/internal/apiserver/application/authn/register"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	req "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/request"
	resp "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/response"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

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

	registerReq, err := registerWechatRequestFromHTTP(reqBody)
	if err != nil {
		h.Error(c, err)
		return
	}

	result, err := h.registerService.Register(c.Request.Context(), registerReq)
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, registerResultToResponse(result))
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
