package handler

import (
	challengeapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/challenge"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
)

// AuthHandler 认证 HTTP 处理器
type AuthHandler struct {
	*BaseHandler
	loginService login.LoginApplicationService
	tokenService token.TokenApplicationService
	challenge    challengeapp.Service
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(
	loginService login.LoginApplicationService,
	tokenService token.TokenApplicationService,
	challenge challengeapp.Service,
) *AuthHandler {
	return &AuthHandler{
		BaseHandler:  NewBaseHandler(),
		loginService: loginService,
		tokenService: tokenService,
		challenge:    challenge,
	}
}
