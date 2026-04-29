package handler

import (
	appAccount "github.com/FangcunMount/iam/internal/apiserver/application/authn/account"
	appRegister "github.com/FangcunMount/iam/internal/apiserver/application/authn/register"
	_ "github.com/FangcunMount/iam/pkg/core"
)

// AccountHandler 账户管理 HTTP Handler
type AccountHandler struct {
	*BaseHandler
	accountService  appAccount.AccountApplicationService
	registerService appRegister.RegisterApplicationService
}

// NewAccountHandler 创建账户处理器
func NewAccountHandler(
	accountService appAccount.AccountApplicationService,
	registerService appRegister.RegisterApplicationService,
) *AccountHandler {
	return &AccountHandler{
		BaseHandler:     NewBaseHandler(),
		accountService:  accountService,
		registerService: registerService,
	}
}
