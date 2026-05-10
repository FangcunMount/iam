package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	linkingapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/linking"
	req "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authn/request"
	resp "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authn/response"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/FangcunMount/iam/v2/internal/pkg/requestctx"
)

type LoginIdentityHandler struct {
	*BaseHandler
	linking linkingapp.Service
}

func NewLoginIdentityHandler(linking linkingapp.Service) *LoginIdentityHandler {
	return &LoginIdentityHandler{
		BaseHandler: NewBaseHandler(),
		linking:     linking,
	}
}

// List 列出当前用户登录身份。
// @Summary 列出当前用户登录身份
// @Tags 认证
// @Produce json
// @Security BearerAuth
// @Success 200 {object} resp.LoginIdentityListResponse "当前用户已绑定的登录身份"
// @Router /authn/login-identities [get]
func (h *LoginIdentityHandler) List(c *gin.Context) {
	userID, err := requestctx.RequiredUserID(c)
	if err != nil {
		h.Error(c, err)
		return
	}
	identities, err := h.linking.List(c.Request.Context(), userID)
	if err != nil {
		h.Error(c, err)
		return
	}
	items := make([]resp.LoginIdentityResponse, 0, len(identities))
	for _, identity := range identities {
		items = append(items, loginIdentityViewToResponse(identity))
	}
	h.Success(c, resp.LoginIdentityListResponse{Items: items})
}

// SendPhoneLinkChallenge 发送手机号绑定验证码。
// @Summary 发送手机号绑定验证码
// @Tags 认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body req.LinkPhoneChallengeRequest true "手机号"
// @Success 200 {object} resp.MessageResponse "已发送"
// @Router /authn/login-identities/phone/challenge [post]
func (h *LoginIdentityHandler) SendPhoneLinkChallenge(c *gin.Context) {
	userID, err := requestctx.RequiredUserID(c)
	if err != nil {
		h.Error(c, err)
		return
	}

	var body req.LinkPhoneChallengeRequest
	if err := h.BindJSON(c, &body); err != nil {
		h.Error(c, err)
		return
	}
	if err := body.Validate(); err != nil {
		h.Error(c, err)
		return
	}
	if err := h.linking.SendPhoneLinkChallenge(c.Request.Context(), userID, body.Phone); err != nil {
		h.Error(c, err)
		return
	}
	h.Success(c, resp.MessageResponse{Message: "verification code sent"})
}

// LinkPhone 绑定手机号登录身份。
// @Summary 绑定手机号登录身份
// @Tags 认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body req.LinkPhoneRequest true "手机号与验证码"
// @Success 200 {object} resp.LinkLoginIdentityResponse "绑定结果"
// @Router /authn/login-identities/phone [post]
func (h *LoginIdentityHandler) LinkPhone(c *gin.Context) {
	userID, err := requestctx.RequiredUserID(c)
	if err != nil {
		h.Error(c, err)
		return
	}
	var body req.LinkPhoneRequest
	if err := h.BindJSON(c, &body); err != nil {
		h.Error(c, err)
		return
	}
	if err := body.Validate(); err != nil {
		h.Error(c, err)
		return
	}
	result, err := h.linking.LinkPhone(c.Request.Context(), linkingapp.LinkPhoneCommand{
		UserID:  userID,
		Phone:   body.Phone,
		OTPCode: body.OTPCode,
	})
	if err != nil {
		h.Error(c, err)
		return
	}
	h.Success(c, linkResultToResponse(result))
}

// LinkWechatMiniProgram 绑定微信小程序登录身份。
// @Summary 绑定微信小程序登录身份
// @Tags 认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body req.LinkWechatMiniProgramRequest true "微信小程序 code"
// @Success 200 {object} resp.LinkLoginIdentityResponse "绑定结果"
// @Router /authn/login-identities/wechat-miniprogram [post]
func (h *LoginIdentityHandler) LinkWechatMiniProgram(c *gin.Context) {
	userID, err := requestctx.RequiredUserID(c)
	if err != nil {
		h.Error(c, err)
		return
	}
	var body req.LinkWechatMiniProgramRequest
	if err := h.BindJSON(c, &body); err != nil {
		h.Error(c, err)
		return
	}
	if err := body.Validate(); err != nil {
		h.Error(c, err)
		return
	}
	result, err := h.linking.LinkWechatMini(c.Request.Context(), linkingapp.LinkWechatMiniCommand{
		UserID: userID,
		AppID:  body.AppID,
		Code:   body.Code,
	})
	if err != nil {
		h.Error(c, err)
		return
	}
	h.Success(c, linkResultToResponse(result))
}

// LinkWecom 绑定企业微信登录身份。
// @Summary 绑定企业微信登录身份
// @Tags 认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body req.LinkWecomRequest true "企业微信 code"
// @Success 200 {object} resp.LinkLoginIdentityResponse "绑定结果"
// @Router /authn/login-identities/wecom [post]
func (h *LoginIdentityHandler) LinkWecom(c *gin.Context) {
	userID, err := requestctx.RequiredUserID(c)
	if err != nil {
		h.Error(c, err)
		return
	}
	var body req.LinkWecomRequest
	if err := h.BindJSON(c, &body); err != nil {
		h.Error(c, err)
		return
	}
	if err := body.Validate(); err != nil {
		h.Error(c, err)
		return
	}
	result, err := h.linking.LinkWecom(c.Request.Context(), linkingapp.LinkWecomCommand{
		UserID: userID,
		CorpID: body.CorpID,
		Code:   body.Code,
	})
	if err != nil {
		h.Error(c, err)
		return
	}
	h.Success(c, linkResultToResponse(result))
}

// Unlink 解绑登录身份。
// @Summary 解绑登录身份
// @Tags 认证
// @Produce json
// @Security BearerAuth
// @Param id path string true "登录身份 ID"
// @Success 200 {object} resp.MessageResponse "已解绑"
// @Router /authn/login-identities/{id} [delete]
func (h *LoginIdentityHandler) Unlink(c *gin.Context) {
	userID, err := requestctx.RequiredUserID(c)
	if err != nil {
		h.Error(c, err)
		return
	}
	id, err := meta.ParseID(strings.TrimSpace(c.Param("id")))
	if err != nil {
		h.Error(c, err)
		return
	}
	if err := h.linking.Unlink(c.Request.Context(), linkingapp.UnlinkCommand{
		UserID:          userID,
		LoginIdentityID: id,
	}); err != nil {
		h.Error(c, err)
		return
	}
	h.Success(c, resp.MessageResponse{Message: "login identity unlinked"})
}

func loginIdentityViewToResponse(identity linkingapp.LoginIdentityView) resp.LoginIdentityResponse {
	return resp.LoginIdentityResponse{
		ID:               identity.ID.String(),
		Provider:         string(identity.Provider),
		Realm:            identity.Realm,
		Identifier:       identity.Identifier,
		GlobalIdentifier: identity.GlobalIdentifier,
		Status:           string(identity.Status),
		VerifiedAt:       identity.VerifiedAt,
		LinkedAt:         identity.LinkedAt,
	}
}

func linkResultToResponse(result *linkingapp.LinkResult) resp.LinkLoginIdentityResponse {
	return resp.LinkLoginIdentityResponse{
		LoginIdentity: loginIdentityViewToResponse(linkingapp.LoginIdentityView{
			ID:               result.Identity.ID,
			UserID:           result.Identity.UserID,
			Provider:         result.Identity.Provider,
			Realm:            result.Identity.Realm,
			Identifier:       result.Identity.Identifier,
			GlobalIdentifier: result.Identity.GlobalIdentifier,
			Status:           result.Identity.Status,
			VerifiedAt:       result.Identity.VerifiedAt,
			LinkedAt:         result.Identity.LinkedAt,
		}),
		Reused: result.Reused,
	}
}
