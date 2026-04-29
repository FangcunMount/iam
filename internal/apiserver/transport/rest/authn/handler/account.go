package handler

import (
	appAccount "github.com/FangcunMount/iam/internal/apiserver/application/authn/account"
	appOnboarding "github.com/FangcunMount/iam/internal/apiserver/application/authn/onboarding"
	_ "github.com/FangcunMount/iam/pkg/core"
)

// AccountHandler 账户管理 HTTP Handler
type AccountHandler struct {
	*BaseHandler
	accountService   appAccount.AccountApplicationService
	accountOnboarder appOnboarding.AccountOnboarder
}

// NewAccountHandler 创建账户处理器
func NewAccountHandler(
	accountService appAccount.AccountApplicationService,
	accountOnboarder appOnboarding.AccountOnboarder,
) *AccountHandler {
	return &AccountHandler{
		BaseHandler:      NewBaseHandler(),
		accountService:   accountService,
		accountOnboarder: accountOnboarder,
	}
}
