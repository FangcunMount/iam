package assembler

import (
	"context"
	"time"

	challengeApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/challenge"
	jwksApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/jwks"
	linkingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/linking"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login"
	onboardingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/onboarding"
	sessionApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/session"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	authzAuthorizationApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/authorization"
	authzPolicyApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/policy"
	authzPolicylintApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/policylint"
	authzResourceApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/resource"
	authzRoleApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/role"
	authzRolebindingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/rolebinding"
	appprofile "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profile"
	appprofilelink "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profilelink"
	appuser "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/user"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/idp/wechatapp"
	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/subject"
	wechatappDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/middleware/authn"
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
	LoginIdentityOnboarder onboardingApp.LoginIdentityOnboarder
	LoginIdentityLinking   linkingApp.Service
	LoginService           login.LoginApplicationService
	ChallengeService       challengeApp.Service
	TokenService           token.TokenApplicationService
	SessionService         sessionApp.SessionApplicationService
	KeyManagementApp       *jwksApp.KeyManagementAppService
	KeyPublishApp          *jwksApp.KeyPublishAppService
	KeyRotationApp         *jwksApp.KeyRotationAppService
}

type AuthnRuntimeCapabilities struct {
	RotationScheduler KeyRotationScheduler
}

type AuthzApplicationCapabilities struct {
	ResourceCatalog             authzResourceApp.Catalog
	ResourceDirectory           authzResourceApp.Directory
	RoleCatalog                 authzRoleApp.Catalog
	RoleDirectory               authzRoleApp.Directory
	PermissionCommands          authzPolicyApp.PermissionCommands
	PermissionReader            authzPolicyApp.PermissionReader
	PolicyLinter                *authzPolicylintApp.Linter
	RoleBindingCommands         authzRolebindingApp.Commands
	RoleBindingDirectory        authzRolebindingApp.Directory
	RouteAuthorization          authn.RouteAuthorizationRuntime
	RuntimeHealth               AuthzRuntimeHealthReporter
	AuthorizationChecker        *authzAuthorizationApp.Checker
	AuthorizationSnapshotReader *authzAuthorizationApp.SnapshotReader
}

type RoleNameReader interface {
	RoleNamesForSubject(ctx context.Context, subject subject.Ref, tenantID string) ([]string, error)
}

type AuthzRuntimeHealthReporter interface {
	ReloadHealth() (bool, error, time.Time)
	RuntimeHealthDetails() map[string]any
}

type UserApplicationCapabilities struct {
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

type IDPApplicationCapabilities struct {
	WechatAppService           wechatapp.WechatAppApplicationService
	WechatAppCredentialService wechatapp.WechatAppCredentialApplicationService
	WechatAppTokenService      wechatapp.WechatAppTokenApplicationService
	WechatAppRepository        wechatappDomain.Repository
	SecretVault                wechatappDomain.SecretVault
}

type SuggestApplicationCapabilities struct {
	Service appsuggest.ProfileSuggestor
}

type SuggestRuntimeCapabilities struct {
	Cleanup func() error
}
