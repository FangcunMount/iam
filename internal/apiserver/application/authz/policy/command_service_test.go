package policy

import (
	"context"
	"errors"
	"testing"

	authzuow "github.com/FangcunMount/iam/internal/apiserver/application/authz/uow"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/internal/apiserver/eventing"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/FangcunMount/iam/pkg/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyCommandServiceAddPolicyRule_CommitsFactsWhenRuntimeReloadFails(t *testing.T) {
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
	ruleStore := &policyRuleStoreStub{}
	runtime := &policyCasbinAdapterStub{loadErr: errors.New("reload failed")}
	stager := &policyEventStagerStub{}

	service := NewPolicyCommandService(
		policyDomain.NewValidator(roleRepo, resourceRepo),
		&policyUowStub{tx: authzuow.TxRepositories{
			Roles:          roleRepo,
			Resources:      resourceRepo,
			PolicyVersions: versionRepo,
			RuleStore:      ruleStore,
			Events:         stager,
		}},
		runtime,
	)

	err := service.AddPolicyRule(context.Background(), policyDomain.AddPolicyRuleCommand{
		RoleID:     10,
		ResourceID: resourceDomain.NewResourceID(20),
		Action:     "read",
		TenantID:   "tenant-a",
		ChangedBy:  "1",
		Reason:     "grant read",
	})
	require.NoError(t, err)
	require.Len(t, ruleStore.policyAdds, 1)
	assert.Equal(t, "role:iam:admin", ruleStore.policyAdds[0].Sub)
	assert.Equal(t, "tenant-a", ruleStore.policyAdds[0].Dom)
	assert.Equal(t, "iam:user:*", ruleStore.policyAdds[0].Obj)
	assert.Equal(t, "read", ruleStore.policyAdds[0].Act)
	assert.Equal(t, 1, versionRepo.incrementCalls)
	require.Len(t, stager.events, 1)
	assert.Equal(t, eventing.AuthzVersionChanged, stager.events[0].EventType())
	assert.Equal(t, 3, runtime.loadCalls)
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
func (r *resourceRepoStub) List(context.Context, resourceDomain.ListResourcesQuery) ([]*resourceDomain.Resource, int64, error) {
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

type policyRuleStoreStub struct {
	policyAdds []policyDomain.PolicyRule
}

func (r *policyRuleStoreStub) AddPolicy(_ context.Context, rules ...policyDomain.PolicyRule) error {
	r.policyAdds = append(r.policyAdds, rules...)
	return nil
}
func (r *policyRuleStoreStub) RemovePolicy(context.Context, ...policyDomain.PolicyRule) error {
	return nil
}
func (r *policyRuleStoreStub) AddGroupingPolicy(context.Context, ...policyDomain.GroupingRule) error {
	return nil
}
func (r *policyRuleStoreStub) RemoveGroupingPolicy(context.Context, ...policyDomain.GroupingRule) error {
	return nil
}

type policyCasbinAdapterStub struct {
	loadErr   error
	loadCalls int
}

func (s *policyCasbinAdapterStub) AddPolicy(context.Context, ...policyDomain.PolicyRule) error {
	return nil
}
func (s *policyCasbinAdapterStub) RemovePolicy(context.Context, ...policyDomain.PolicyRule) error {
	return nil
}
func (s *policyCasbinAdapterStub) AddGroupingPolicy(context.Context, ...policyDomain.GroupingRule) error {
	return nil
}
func (s *policyCasbinAdapterStub) RemoveGroupingPolicy(context.Context, ...policyDomain.GroupingRule) error {
	return nil
}
func (s *policyCasbinAdapterStub) GetPoliciesByRole(context.Context, string, string) ([]policyDomain.PolicyRule, error) {
	return nil, nil
}
func (s *policyCasbinAdapterStub) GetGroupingsBySubject(context.Context, string, string) ([]policyDomain.GroupingRule, error) {
	return nil, nil
}
func (s *policyCasbinAdapterStub) LoadPolicy(context.Context) error {
	s.loadCalls++
	return s.loadErr
}
func (s *policyCasbinAdapterStub) Enforce(context.Context, string, string, string, string) (bool, error) {
	return false, nil
}
func (s *policyCasbinAdapterStub) GetRolesForUser(context.Context, string, string) ([]string, error) {
	return nil, nil
}
func (s *policyCasbinAdapterStub) GetImplicitRolesForUser(context.Context, string, string) ([]string, error) {
	return nil, nil
}
func (s *policyCasbinAdapterStub) GetImplicitPermissionsForUser(context.Context, string, string) ([]policyDomain.PolicyRule, error) {
	return nil, nil
}
func (s *policyCasbinAdapterStub) InvalidateCache() {}
