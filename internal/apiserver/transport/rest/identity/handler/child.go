package handler

import (
	appchild "github.com/FangcunMount/iam/internal/apiserver/application/uc/child"
	appregistration "github.com/FangcunMount/iam/internal/apiserver/application/uc/registration"
	_ "github.com/FangcunMount/iam/pkg/core" // imported for swagger
)

// ChildHandler 儿童档案 REST 处理器
type ChildHandler struct {
	*BaseHandler
	registrationApp appregistration.ChildRegistrationService
	childAccess     appchild.ChildAccessApplicationService
	childQuery      appchild.ChildQueryApplicationService
}

// NewChildHandler 创建儿童档案处理器
func NewChildHandler(
	registrationApp appregistration.ChildRegistrationService,
	childAccess appchild.ChildAccessApplicationService,
	childQuery appchild.ChildQueryApplicationService,
) *ChildHandler {
	return &ChildHandler{
		BaseHandler:     NewBaseHandler(),
		registrationApp: registrationApp,
		childAccess:     childAccess,
		childQuery:      childQuery,
	}
}
