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
	tokenVerifier       token.Verifier
	tokenRevoker        token.Revoker
	loginPhoneOTPSender challengeapp.LoginPhoneOTPSender
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(
	sessionService session.ApplicationService,
	tokens token.Capabilities,
	loginPhoneOTPSender challengeapp.LoginPhoneOTPSender,
) *AuthHandler {
	return &AuthHandler{
		BaseHandler:         NewBaseHandler(),
		sessionService:      sessionService,
		tokenVerifier:       tokens.Verifier,
		tokenRevoker:        tokens.Revoker,
		loginPhoneOTPSender: loginPhoneOTPSender,
	}
}
