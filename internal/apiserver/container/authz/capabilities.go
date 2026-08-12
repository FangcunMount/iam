package authz

import (
	"context"
	"time"

	assignmentAuthApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignmentauth"
	authorizationApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	policyApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policy"
	policylintApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policylint"
	resourceApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/resource"
	roleApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/role"
	bindingApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/rolebinding"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/pkg/middleware/authn"
)

// RoleNameReader resolves role names for a subject within a tenant.
// The method shape matches identity.RoleNameReader without importing identity.
type RoleNameReader interface {
	RoleNamesForSubject(ctx context.Context, subject subject.Ref, tenantID string) ([]string, error)
}

// RuntimeHealthReporter exposes authz runtime reload health.
type RuntimeHealthReporter interface {
	ReloadHealth() (bool, error, time.Time)
	RuntimeHealthDetails() map[string]any
}

// ApplicationCapabilities contains authz application collaborators used
// by transports without exposing concrete transport objects from the module.
type ApplicationCapabilities struct {
	ResourceCatalog             resourceApp.Catalog
	ResourceDirectory           resourceApp.Directory
	RoleCatalog                 roleApp.Catalog
	RoleDirectory               roleApp.Directory
	PermissionCommands          policyApp.PermissionCommands
	PermissionReader            policyApp.PermissionReader
	PolicyLinter                *policylintApp.Linter
	RoleBindingCommands         bindingApp.Commands
	RoleBindingDirectory        bindingApp.Directory
	RouteAuthorization          authn.RouteAuthorizationRuntime
	RuntimeHealth               RuntimeHealthReporter
	AuthorizationChecker        *authorizationApp.Checker
	AuthorizationSnapshotReader *authorizationApp.SnapshotReader
	AssignmentRequestAuthorizer assignmentAuthApp.Authorizer
}
