package handler

import (
	appAccount "github.com/FangcunMount/iam/internal/apiserver/application/authn/account"
	appOnboarding "github.com/FangcunMount/iam/internal/apiserver/application/authn/onboarding"
	_ "github.com/FangcunMount/iam/pkg/core"
)

// AccountHandler 账户管理 HTTP Handler
type AccountHandler struct {
	*BaseHandler
	accountDirectory appAccount.AccountDirectory
	profileEditor    appAccount.AccountProfileEditor
	statusChanger    appAccount.AccountStatusChanger
	accountOnboarder appOnboarding.AccountOnboarder
}

// NewAccountHandler 创建账户处理器
func NewAccountHandler(
	accountService appAccount.AccountApplicationService,
	accountOnboarder appOnboarding.AccountOnboarder,
) *AccountHandler {
	return NewAccountHandlerWithRoles(accountService, accountService, accountService, accountOnboarder)
}

// NewAccountHandlerWithRoles 使用窄应用角色构造账户处理器。
func NewAccountHandlerWithRoles(
	accountDirectory appAccount.AccountDirectory,
	profileEditor appAccount.AccountProfileEditor,
	statusChanger appAccount.AccountStatusChanger,
	accountOnboarder appOnboarding.AccountOnboarder,
) *AccountHandler {
	return &AccountHandler{
		BaseHandler:      NewBaseHandler(),
		accountDirectory: accountDirectory,
		profileEditor:    profileEditor,
		statusChanger:    statusChanger,
		accountOnboarder: accountOnboarder,
	}
}
