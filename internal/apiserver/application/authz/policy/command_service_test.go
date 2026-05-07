package policy

import (
	"context"
	"errors"
	"testing"

	authzuow "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/uow"
	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	policyDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v2/internal/apiserver/eventing"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/FangcunMount/iam/v2/pkg/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyCommandServiceAddPermission_CommitsFactsWhenRuntimeReloadFails(t *testing.T) {
	roleRepo := &policyRoleRepoStub{
		role: &roleDomain.Role{
			ID:       meta.FromUint64(10),
			Name:     "iam:admin",
			TenantID: "tenant-a",
		},
	}
	resourceRepo := &resourceRepoStub{
		resource: &resourceDomain.Resource{
			ID:      resourceDomain.NewResourceID(20),
			Key:     "iam:user:*",
			Actions: []string{"read"},
		},
	}
	versionRepo := &policyVersionRepoForCommandStub{}
	ruleStore := &policyAuthorizationFactStoreStub{}
	runtime := &policyCasbinAdapterStub{loadErr: errors.New("reload failed")}
	stager := &policyEventStagerStub{}

	service := NewPolicyCommandService(
		policyDomain.NewValidator(roleRepo, resourceRepo),
		&policyUowStub{tx: authzuow.TxRepositories{
			Roles:              roleRepo,
			Resources:          resourceRepo,
			PolicyVersions:     versionRepo,
			AuthorizationFacts: ruleStore,
			Events:             stager,
		}},
		runtime,
	)

	err := service.AddPermission(context.Background(), AddPermissionCommand{
		RoleID:     meta.FromUint64(10),
		ResourceID: resourceDomain.NewResourceID(20),
		Action:     "read",
		TenantID:   "tenant-a",
		ChangedBy:  "1",
		Reason:     "grant read",
	})
	require.NoError(t, err)
	require.Len(t, ruleStore.policyAdds, 1)
	assert.Equal(t, "iam:admin", ruleStore.policyAdds[0].RoleName)
	assert.Equal(t, "tenant-a", ruleStore.policyAdds[0].TenantID)
	assert.Equal(t, "iam:user:*", ruleStore.policyAdds[0].ResourceKey)
	assert.Equal(t, "read", ruleStore.policyAdds[0].Action)
	assert.Equal(t, authzDomain.DefaultScope(), ruleStore.policyAdds[0].Scope)
	assert.Equal(t, 1, versionRepo.incrementCalls)
	require.Len(t, stager.events, 1)
	assert.Equal(t, eventing.AuthzVersionChanged, stager.events[0].EventType())
	assert.Equal(t, 3, runtime.loadCalls)
}

func TestPolicyCommandServiceAddPermission_ValidatesResourceScopeKind(t *testing.T) {
	roleRepo := &policyRoleRepoStub{
		role: &roleDomain.Role{
			ID:       meta.FromUint64(10),
			Name:     "iam:admin",
			TenantID: "tenant-a",
		},
	}
	resourceRepo := &resourceRepoStub{
		resource: &resourceDomain.Resource{
			ID:         resourceDomain.NewResourceID(20),
			Key:        "iam:user:*",
			Actions:    []string{"read"},
			ScopeKinds: []authzDomain.ScopeKind{authzDomain.ScopeKindAll, authzDomain.ScopeKindOrigin},
		},
	}
	ruleStore := &policyAuthorizationFactStoreStub{}
	service := NewPolicyCommandService(
		policyDomain.NewValidator(roleRepo, resourceRepo),
		&policyUowStub{tx: authzuow.TxRepositories{
			Roles:              roleRepo,
			Resources:          resourceRepo,
			PolicyVersions:     &policyVersionRepoForCommandStub{},
			AuthorizationFacts: ruleStore,
			Events:             &policyEventStagerStub{},
		}},
		&policyCasbinAdapterStub{},
	)
	scope, err := authzDomain.NewScope(authzDomain.ScopeKindOrigin, "1")
	require.NoError(t, err)

	err = service.AddPermission(context.Background(), AddPermissionCommand{
		RoleID:     meta.FromUint64(10),
		ResourceID: resourceDomain.NewResourceID(20),
		Action:     "read",
		Scope:      scope,
		TenantID:   "tenant-a",
		ChangedBy:  "1",
	})

	require.NoError(t, err)
	require.Len(t, ruleStore.policyAdds, 1)
	assert.Equal(t, scope, ruleStore.policyAdds[0].Scope)
}

func TestPolicyCommandServiceAddPermission_RejectsUnsupportedScopeKind(t *testing.T) {
	roleRepo := &policyRoleRepoStub{
		role: &roleDomain.Role{
			ID:       meta.FromUint64(10),
			Name:     "iam:admin",
			TenantID: "tenant-a",
		},
	}
	resourceRepo := &resourceRepoStub{
		resource: &resourceDomain.Resource{
			ID:      resourceDomain.NewResourceID(20),
			Key:     "iam:user:*",
			Actions: []string{"read"},
		},
	}
	service := NewPolicyCommandService(
		policyDomain.NewValidator(roleRepo, resourceRepo),
		&policyUowStub{tx: authzuow.TxRepositories{
			Roles:              roleRepo,
			Resources:          resourceRepo,
			PolicyVersions:     &policyVersionRepoForCommandStub{},
			AuthorizationFacts: &policyAuthorizationFactStoreStub{},
			Events:             &policyEventStagerStub{},
		}},
		&policyCasbinAdapterStub{},
	)
	scope, err := authzDomain.NewScope(authzDomain.ScopeKindOrigin, "1")
	require.NoError(t, err)

	err = service.AddPermission(context.Background(), AddPermissionCommand{
		RoleID:     meta.FromUint64(10),
		ResourceID: resourceDomain.NewResourceID(20),
		Action:     "read",
		Scope:      scope,
		TenantID:   "tenant-a",
		ChangedBy:  "1",
	})

	require.Error(t, err)
}

type policyUowStub struct {
	tx authzuow.TxRepositories
}

func (u *policyUowStub) WithinTx(ctx context.Context, fn func(txCtx context.Context, tx authzuow.TxRepositories) error) error {
	return fn(ctx, u.tx)
}

type policyEventStagerStub struct {
	events []event.DomainEvent
}

func (s *policyEventStagerStub) Stage(_ context.Context, events ...event.DomainEvent) error {
	s.events = append(s.events, events...)
	return nil
}

type policyRoleRepoStub struct {
	role *roleDomain.Role
}

func (r *policyRoleRepoStub) Create(context.Context, *roleDomain.Role) error { return nil }
func (r *policyRoleRepoStub) Update(context.Context, *roleDomain.Role) error { return nil }
func (r *policyRoleRepoStub) Delete(context.Context, meta.ID) error          { return nil }
func (r *policyRoleRepoStub) FindByID(context.Context, meta.ID) (*roleDomain.Role, error) {
	return r.role, nil
}
func (r *policyRoleRepoStub) FindByName(context.Context, string, string) (*roleDomain.Role, error) {
	return r.role, nil
}
func (r *policyRoleRepoStub) List(context.Context, string, int, int) ([]*roleDomain.Role, int64, error) {
	return nil, 0, nil
}

type resourceRepoStub struct {
	resource *resourceDomain.Resource
}

func (r *resourceRepoStub) Create(context.Context, *resourceDomain.Resource) error  { return nil }
func (r *resourceRepoStub) Update(context.Context, *resourceDomain.Resource) error  { return nil }
func (r *resourceRepoStub) Delete(context.Context, resourceDomain.ResourceID) error { return nil }
func (r *resourceRepoStub) FindByID(context.Context, resourceDomain.ResourceID) (*resourceDomain.Resource, error) {
	return r.resource, nil
}
func (r *resourceRepoStub) FindByKey(context.Context, string) (*resourceDomain.Resource, error) {
	return r.resource, nil
}
func (r *resourceRepoStub) List(context.Context, resourceDomain.ResourceFilter) ([]*resourceDomain.Resource, int64, error) {
	return nil, 0, nil
}
func (r *resourceRepoStub) ValidateAction(context.Context, string, string) (bool, error) {
	return true, nil
}

type policyVersionRepoForCommandStub struct {
	currentVersion int64
	incrementCalls int
}

func (r *policyVersionRepoForCommandStub) GetOrCreate(_ context.Context, tenantID string) (*policyDomain.PolicyVersion, error) {
	version := policyDomain.NewPolicyVersion(tenantID, r.currentVersion)
	return &version, nil
}
func (r *policyVersionRepoForCommandStub) Increment(_ context.Context, tenantID, changedBy, reason string) (*policyDomain.PolicyVersion, error) {
	r.incrementCalls++
	r.currentVersion++
	version := policyDomain.NewPolicyVersion(
		tenantID,
		r.currentVersion,
		policyDomain.WithChangedBy(changedBy),
		policyDomain.WithReason(reason),
	)
	return &version, nil
}
func (r *policyVersionRepoForCommandStub) GetCurrent(_ context.Context, tenantID string) (*policyDomain.PolicyVersion, error) {
	version := policyDomain.NewPolicyVersion(tenantID, r.currentVersion)
	return &version, nil
}

type policyAuthorizationFactStoreStub struct {
	policyAdds []authzDomain.Permission
}

func (r *policyAuthorizationFactStoreStub) AddPermission(_ context.Context, permission authzDomain.Permission) error {
	r.policyAdds = append(r.policyAdds, permission)
	return nil
}
func (r *policyAuthorizationFactStoreStub) RemovePermission(context.Context, authzDomain.Permission) error {
	return nil
}
func (r *policyAuthorizationFactStoreStub) AddRoleBinding(context.Context, authzDomain.RoleBinding) error {
	return nil
}
func (r *policyAuthorizationFactStoreStub) RemoveRoleBinding(context.Context, authzDomain.RoleBinding) error {
	return nil
}

type policyCasbinAdapterStub struct {
	loadErr   error
	loadCalls int
}

func (s *policyCasbinAdapterStub) LoadPolicy(context.Context) error {
	s.loadCalls++
	return s.loadErr
}
func (s *policyCasbinAdapterStub) InvalidateCache() {}
