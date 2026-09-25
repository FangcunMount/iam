package authz

import (
	"context"
	"time"

	assignmentApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/assignment"
	assignmentAdmissionApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/assignmentadmission"
	authorizationApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/authorization"
	permissionGrantApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/permissiongrant"
	policyVersionApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policyversion"
	resourceApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/resource"
	roleApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/role"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/subject"
)

// EffectiveRoleReader resolves direct and inherited roles for a subject.
// The method shape matches identity.EffectiveRoleReader without importing identity.
type EffectiveRoleReader interface {
	EffectiveRoleNamesForSubject(ctx context.Context, subject subject.Ref) ([]string, error)
}

// RuntimeHealthReporter exposes authz runtime reload health.
type RuntimeHealthReporter interface {
	ReloadHealth() (bool, error, time.Time)
	RuntimeHealthDetails() map[string]any
}

// ApplicationCapabilities contains authz application collaborators used
// by transports without exposing concrete transport objects from the module.
type ApplicationCapabilities struct {
	ResourceCatalog              resourceApp.Catalog
	ResourceDirectory            resourceApp.Directory
	RoleCatalog                  roleApp.Catalog
	RoleDirectory                roleApp.Directory
	PermissionGrantService       *permissionGrantApp.Service
	AssignmentCommands           assignmentApp.Commands
	AssignmentDirectory          assignmentApp.Directory
	RoutePermissionChecker       authorizationApp.RoutePermissionChecker
	RuntimeHealth                RuntimeHealthReporter
	AuthorizationDecisions       *authorizationApp.DecisionService
	AuthorizationSnapshotReader  *authorizationApp.SnapshotReader
	CommittedPolicyVersionReader *policyVersionApp.Reader
	AssignmentAdmissionPolicy    assignmentAdmissionApp.Policy
}
