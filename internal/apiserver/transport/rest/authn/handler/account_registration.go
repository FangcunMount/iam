package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	appOnboarding "github.com/FangcunMount/iam/internal/apiserver/application/authn/onboarding"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	req "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/request"
	resp "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/response"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// SignUpWithWeChatMiniProgram 微信小程序账号开通。
// @Summary 微信小程序账号开通
// @Description 使用微信 JS Code 开通账号，服务端根据 AppID 查询 AppSecret 并自动调用 code2session 获取 OpenID 和 UnionID
// @Tags 账户管理
// @Accept json
// @Produce json
// @Param request body req.SignUpWithWeChatMiniProgramRequest true "微信小程序账号开通请求"
// @Success 201 {object} resp.SignupResult "开通成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 409 {object} map[string]interface{} "用户已存在"
// @Router /authn/signups/wechat-miniprogram [post]
func (h *AccountHandler) SignUpWithWeChatMiniProgram(c *gin.Context) {
	var reqBody req.SignUpWithWeChatMiniProgramRequest
	if err := h.BindJSON(c, &reqBody); err != nil {
		h.Error(c, err)
		return
	}

	onboardingReq, err := wechatMiniProgramSignupRequestFromHTTP(reqBody)
	if err != nil {
		h.Error(c, err)
		return
	}

	result, err := h.accountOnboarder.Onboard(c.Request.Context(), onboardingReq)
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, signupResultToResponse(result))
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
	onboardingReq := appOnboarding.OnboardingRequest{
		Name:           strings.TrimSpace(reqBody.Name),
		Phone:          phone,
		Email:          email,
		AccountType:    domain.TypeMockConsumer,
		CredentialType: appOnboarding.CredTypePassword,
		Password:       &password,
		Profile:        reqBody.Profile,
		Meta:           reqBody.Meta,
	}

	result, err := h.accountOnboarder.Onboard(c.Request.Context(), onboardingReq)
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
