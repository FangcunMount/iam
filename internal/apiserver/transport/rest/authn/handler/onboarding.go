package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	signupApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signup"
	req "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authn/request"
	resp "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authn/response"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// OnboardingHandler exposes /authn/signups and internal mock-consumer ensure endpoints.
type OnboardingHandler struct {
	*BaseHandler
	signupService signupApp.SignupService
}

func NewOnboardingHandler(signupService signupApp.SignupService) *OnboardingHandler {
	return &OnboardingHandler{
		BaseHandler:   NewBaseHandler(),
		signupService: signupService,
	}
}

// SignUpWithWeChatMiniProgram 微信小程序登录身份开通。
// @Summary 微信小程序登录身份开通
// @Description 通过微信小程序 code 建立 User 与 LoginIdentity 绑定。
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body req.SignUpWithWeChatMiniProgramRequest true "微信小程序登录身份开通请求"
// @Success 200 {object} resp.SignupResult "开通成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 409 {object} map[string]interface{} "登录身份已绑定到其他用户"
// @Router /authn/signups/wechat-miniprogram [post]
func (h *OnboardingHandler) SignUpWithWeChatMiniProgram(c *gin.Context) {
	var reqBody req.SignUpWithWeChatMiniProgramRequest
	if err := h.BindJSON(c, &reqBody); err != nil {
		h.Error(c, err)
		return
	}
	if err := reqBody.Validate(); err != nil {
		h.Error(c, err)
		return
	}

	signupReq, err := wechatMiniProgramSignupRequestFromHTTP(reqBody)
	if err != nil {
		h.Error(c, err)
		return
	}

	result, err := h.signupService.SignUp(c.Request.Context(), signupReq)
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, signupResultToResponse(result))
}

// EnsureMockConsumer 内部 mock C 端登录身份 ensure。
func (h *OnboardingHandler) EnsureMockConsumer(c *gin.Context) {
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
	signupReq := signupApp.SignupRequest{
		User: signupApp.SignupUserInput{
			Name:  strings.TrimSpace(reqBody.Name),
			Phone: phone,
			Email: email,
		},
		LoginIdentity: signupApp.MockConsumerUsernameLoginIdentityInput{
			Username: strings.TrimSpace(reqBody.Email),
			Profile:  reqBody.Profile,
			Meta:     reqBody.Meta,
		},
		Credential: &signupApp.SignupCredentialInput{
			Password: &signupApp.PasswordCredentialInput{Plaintext: password},
		},
	}

	result, err := h.signupService.SignUp(c.Request.Context(), signupReq)
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, resp.EnsureMockConsumerResult{
		UserID:          result.UserID.String(),
		LoginIdentityID: result.LoginIdentityID.String(),
		LoginID:         strings.TrimSpace(reqBody.Email),
		IsNewUser:       result.IsNewUser,
		IsNewIdentity:   result.IsNewLoginIdentity,
	})
}

func wechatMiniProgramSignupRequestFromHTTP(reqBody req.SignUpWithWeChatMiniProgramRequest) (signupApp.SignupRequest, error) {
	var phone meta.Phone
	if strings.TrimSpace(reqBody.Phone) != "" {
		parsed, err := meta.NewPhone(strings.TrimSpace(reqBody.Phone))
		if err != nil {
			return signupApp.SignupRequest{}, perrors.WithCode(code.ErrInvalidArgument, "invalid phone: %v", err)
		}
		phone = parsed
	}
	var email meta.Email
	if strings.TrimSpace(reqBody.Email) != "" {
		parsed, err := meta.NewEmail(strings.TrimSpace(reqBody.Email))
		if err != nil {
			return signupApp.SignupRequest{}, perrors.WithCode(code.ErrInvalidArgument, "invalid email: %v", err)
		}
		email = parsed
	}

	appID := strings.TrimSpace(reqBody.AppID)
	jsCode := strings.TrimSpace(reqBody.JsCode)

	profile := make(map[string]string)
	if reqBody.Nickname != nil && strings.TrimSpace(*reqBody.Nickname) != "" {
		profile["nickname"] = strings.TrimSpace(*reqBody.Nickname)
	}
	if reqBody.Avatar != nil && strings.TrimSpace(*reqBody.Avatar) != "" {
		profile["avatar"] = strings.TrimSpace(*reqBody.Avatar)
	}

	metaMap, err := reqBody.MetaJSON()
	if err != nil {
		return signupApp.SignupRequest{}, err
	}

	return signupApp.SignupRequest{
		User: signupApp.SignupUserInput{
			Name:  strings.TrimSpace(reqBody.Name),
			Phone: phone,
			Email: email,
		},
		LoginIdentity: signupApp.WechatMiniLoginIdentityInput{
			AppID:   &appID,
			JsCode:  &jsCode,
			Profile: profile,
			Meta:    metaMap,
		},
	}, nil
}

func signupResultToResponse(result *signupApp.SignupResult) resp.SignupResult {
	var credential *resp.SignupCredential
	if result.Credential != nil {
		credential = &resp.SignupCredential{
			ID:   result.Credential.ID.Uint64(),
			Type: string(result.Credential.Type),
		}
	}
	return resp.SignupResult{
		UserID:          result.UserID.Uint64(),
		UserName:        result.UserName,
		Phone:           result.Phone.String(),
		Email:           result.Email.String(),
		LoginIdentityID: result.LoginIdentityID.Uint64(),
		Credential:      credential,
		IsNewUser:       result.IsNewUser,
		IsNewIdentity:   result.IsNewLoginIdentity,
	}
}
