package handler

import (
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login"
	loginprep "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/loginprep"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
)

// AuthHandler 认证 HTTP 处理器
type AuthHandler struct {
	*BaseHandler
	loginService     login.LoginApplicationService
	tokenService     token.TokenApplicationService
	loginPreparation loginprep.LoginPreparationService
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(
	loginService login.LoginApplicationService,
	tokenService token.TokenApplicationService,
	loginPreparation loginprep.LoginPreparationService,
) *AuthHandler {
	return &AuthHandler{
		BaseHandler:      NewBaseHandler(),
		loginService:     loginService,
		tokenService:     tokenService,
		loginPreparation: loginPreparation,
	}
}
