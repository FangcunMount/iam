package identity

import (
	appprofile "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profile"
	appprofilelink "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profilelink"
	appuser "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/user"
)

// ApplicationCapabilities contains identity application collaborators used
// by transports without exposing concrete transport objects from the module.
type ApplicationCapabilities struct {
	UserCreator          appuser.Creator
	UserEditor           appuser.Editor
	UserStatusChanger    appuser.StatusChanger
	UserDirectory        appuser.Directory
	ProfileDirectory     appprofile.Directory
	MyProfiles           appprofile.MyProfiles
	ProfileLinkCommands  appprofilelink.Commands
	ProfileLinkDirectory appprofilelink.Directory
	MyProfileLinks       appprofilelink.MyProfileLinks
	RoleNames            RoleNameReader
}
