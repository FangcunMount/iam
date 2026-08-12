package handler

import (
	challengeapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/challenge"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/session"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/token"
)

// AuthHandler 认证 HTTP 处理器
type AuthHandler struct {
	*BaseHandler
	sessionService      session.ApplicationService
	tokenService        token.TokenApplicationService
	loginPhoneOTPSender challengeapp.LoginPhoneOTPSender
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(
	sessionService session.ApplicationService,
	tokenService token.TokenApplicationService,
	loginPhoneOTPSender challengeapp.LoginPhoneOTPSender,
) *AuthHandler {
	return &AuthHandler{
		BaseHandler:         NewBaseHandler(),
		sessionService:      sessionService,
		tokenService:        tokenService,
		loginPhoneOTPSender: loginPhoneOTPSender,
	}
}
