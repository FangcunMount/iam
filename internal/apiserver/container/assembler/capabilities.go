package assembler

import (
	"context"

	accountApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/account"
	jwksApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/jwks"
	"github.com/FangcunMount/iam/internal/apiserver/application/authn/login"
	loginprep "github.com/FangcunMount/iam/internal/apiserver/application/authn/loginprep"
	registerApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/register"
	sessionApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/session"
	"github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/internal/apiserver/application/idp/wechatapp"
	appsuggest "github.com/FangcunMount/iam/internal/apiserver/application/suggest"
	appprofile "github.com/FangcunMount/iam/internal/apiserver/application/uc/profile"
	appprofilelink "github.com/FangcunMount/iam/internal/apiserver/application/uc/profilelink"
	appuser "github.com/FangcunMount/iam/internal/apiserver/application/uc/user"
	assignmentDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/assignment"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	wechatappDomain "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/internal/pkg/middleware/authn"
)

// KeyRotationScheduler is the runtime capability exposed by the authn module.
type KeyRotationScheduler interface {
	Start(ctx context.Context) error
	Stop() error
	IsRunning() bool
	TriggerNow(ctx context.Context) error
}

// AuthnApplicationCapabilities contains authn application collaborators used
// by transports without exposing concrete transport objects from assembler.
type AuthnApplicationCapabilities struct {
	AccountService          accountApp.AccountApplicationService
	RegisterService         registerApp.RegisterApplicationService
	LoginService            login.LoginApplicationService
	LoginPreparationService loginprep.LoginPreparationService
	TokenService            token.TokenApplicationService
	SessionService          sessionApp.SessionApplicationService
	KeyManagementApp        *jwksApp.KeyManagementAppService
	KeyPublishApp           *jwksApp.KeyPublishAppService
	KeyRotationApp          *jwksApp.KeyRotationAppService
}

type AuthnRuntimeCapabilities struct {
	RotationScheduler KeyRotationScheduler
}

type AuthzApplicationCapabilities struct {
	ResourceCommander   resourceDomain.Commander
	ResourceQueryer     resourceDomain.Queryer
	RoleCommander       roleDomain.Commander
	RoleQueryer         roleDomain.Queryer
	PolicyCommander     policyDomain.Commander
	PolicyQueryer       policyDomain.Queryer
	AssignmentCommander assignmentDomain.Commander
	AssignmentQueryer   assignmentDomain.Queryer
	Casbin              policyDomain.CasbinAdapter
	RoleRepository      roleDomain.Repository
	PolicyVersionRepo   policyDomain.Repository
}

type UserApplicationCapabilities struct {
	UserService                appuser.UserApplicationService
	UserProfileService         appuser.UserProfileApplicationService
	UserStatusService          appuser.UserStatusApplicationService
	UserQueryService           appuser.UserQueryApplicationService
	ProfileQueryService        appprofile.ProfileQueryApplicationService
	ProfileAccessService       appprofile.ProfileAccessApplicationService
	ProfileLinkService         appprofilelink.ProfileLinkApplicationService
	ProfileLinkQueryService    appprofilelink.ProfileLinkQueryApplicationService
	ProfileLinkAccessService   appprofilelink.ProfileLinkAccessApplicationService
	ProfileRegistrationService appprofile.ProfileRegistrationService
	Casbin                     authn.CasbinEnforcer
}

type IDPApplicationCapabilities struct {
	WechatAppService           wechatapp.WechatAppApplicationService
	WechatAppCredentialService wechatapp.WechatAppCredentialApplicationService
	WechatAppTokenService      wechatapp.WechatAppTokenApplicationService
	WechatAppRepository        wechatappDomain.Repository
	SecretVault                wechatappDomain.SecretVault
}

type SuggestApplicationCapabilities struct {
	Service *appsuggest.Service
}

type SuggestRuntimeCapabilities struct {
	Cleanup func() error
}
