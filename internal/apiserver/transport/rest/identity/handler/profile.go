package handler

import (
	appprofile "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profile"
	_ "github.com/FangcunMount/iam/v2/pkg/core" // imported for swagger
)

// ProfileHandler 档案 REST 处理器。
type ProfileHandler struct {
	*BaseHandler
	myProfiles       appprofile.MyProfiles
	profileDirectory appprofile.Directory
}

// NewProfileHandler 创建档案处理器。
func NewProfileHandler(
	myProfiles appprofile.MyProfiles,
	profileDirectory appprofile.Directory,
) *ProfileHandler {
	return &ProfileHandler{
		BaseHandler:      NewBaseHandler(),
		myProfiles:       myProfiles,
		profileDirectory: profileDirectory,
	}
}
