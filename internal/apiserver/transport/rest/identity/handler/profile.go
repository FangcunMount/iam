package handler

import (
	appprofile "github.com/FangcunMount/iam/internal/apiserver/application/uc/profile"
	_ "github.com/FangcunMount/iam/pkg/core" // imported for swagger
)

// ProfileHandler 档案 REST 处理器。
type ProfileHandler struct {
	*BaseHandler
	registrationApp appprofile.ProfileRegistrationService
	profileAccess   appprofile.ProfileAccessApplicationService
	profileQuery    appprofile.ProfileQueryApplicationService
}

// NewProfileHandler 创建档案处理器。
func NewProfileHandler(
	registrationApp appprofile.ProfileRegistrationService,
	profileAccess appprofile.ProfileAccessApplicationService,
	profileQuery appprofile.ProfileQueryApplicationService,
) *ProfileHandler {
	return &ProfileHandler{
		BaseHandler:     NewBaseHandler(),
		registrationApp: registrationApp,
		profileAccess:   profileAccess,
		profileQuery:    profileQuery,
	}
}
