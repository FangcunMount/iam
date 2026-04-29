package assignment

import (
	"context"
	"errors"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authzuow "github.com/FangcunMount/iam/internal/apiserver/application/authz/uow"
	assignmentDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/assignment"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	roleDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	userDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/apiserver/eventing"
	"github.com/FangcunMount/iam/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/FangcunMount/iam/pkg/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssignmentCommandServiceGrant_RejectsMissingUserWithoutWrites(t *testing.T) {
	roleRepo := &assignmentRoleRepoStub{
		role: &roleDomain.Role{
			ID:       meta.FromUint64(10),
			Name:     "iam:admin",
			TenantID: "tenant-a",
		},
	}
	assignmentRepo := &assignmentRepoStub{}
	userRepo := testhelpers.NewUserRepoStub()
	versionRepo := &policyVersionRepoStub{}
	ruleStore := &ruleStoreStub{}
	runtime := &casbinAdapterStub{}
	stager := &assignmentEventStagerStub{}

	validator := assignmentDomain.NewValidator(assignmentRepo, roleRepo, userRepo)
	service := NewAssignmentCommandService(
		validator,
		&uowStub{tx: authzuow.TxRepositories{
			Assignments:    assignmentRepo,
			Roles:          roleRepo,
			Users:          userRepo,
			PolicyVersions: versionRepo,
			RuleStore:      ruleStore,
			Events:         stager,
		}},
		runtime,
	)

	_, err := service.Grant(context.Background(), assignmentDomain.GrantCommand{
		SubjectType: assignmentDomain.SubjectTypeUser,
		SubjectID:   "123",
		RoleID:      10,
		TenantID:    "tenant-a",
		GrantedBy:   "1",
	})
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrUserNotFound))
	assert.Equal(t, 0, assignmentRepo.createCalls)
	assert.Len(t, ruleStore.groupingAdds, 0)
	assert.Equal(t, 0, versionRepo.incrementCalls)
	assert.Equal(t, 0, runtime.loadCalls)
}

func TestAssignmentCommandServiceGrant_CommitsFactsWhenRuntimeReloadFails(t *testing.T) {
	roleRepo := &assignmentRoleRepoStub{
		role: &roleDomain.Role{
			ID:       meta.FromUint64(10),
			Name:     "iam:admin",
			TenantID: "tenant-a",
		},
	}
	assignmentRepo := &assignmentRepoStub{}
	userRepo := testhelpers.NewUserRepoStub()
	userRepo.UsersByID[123] = &userDomain.User{ID: meta.FromUint64(123)}
	versionRepo := &policyVersionRepoStub{}
	ruleStore := &ruleStoreStub{}
	runtime := &casbinAdapterStub{loadErr: errors.New("reload failed")}
	stager := &assignmentEventStagerStub{}

	validator := assignmentDomain.NewValidator(assignmentRepo, roleRepo, userRepo)
	service := NewAssignmentCommandService(
		validator,
		&uowStub{tx: authzuow.TxRepositories{
			Assignments:    assignmentRepo,
			Roles:          roleRepo,
			Users:          userRepo,
			PolicyVersions: versionRepo,
			RuleStore:      ruleStore,
			Events:         stager,
		}},
		runtime,
	)

	result, err := service.Grant(context.Background(), assignmentDomain.GrantCommand{
		SubjectType: assignmentDomain.SubjectTypeUser,
		SubjectID:   "123",
		RoleID:      10,
		TenantID:    "tenant-a",
		GrantedBy:   "1",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, uint64(1), result.ID.Uint64())
	assert.Equal(t, 1, assignmentRepo.createCalls)
	require.Len(t, ruleStore.groupingAdds, 1)
	assert.Equal(t, "user:123", ruleStore.groupingAdds[0].Sub)
	assert.Equal(t, "role:iam:admin", ruleStore.groupingAdds[0].Role)
	assert.Equal(t, 1, versionRepo.incrementCalls)
	require.Len(t, stager.events, 1)
	assert.Equal(t, eventing.AuthzVersionChanged, stager.events[0].EventType())
	assert.Equal(t, 3, runtime.loadCalls)
}

type uowStub struct {
	tx authzuow.TxRepositories
}

func (u *uowStub) WithinTx(ctx context.Context, fn func(txCtx context.Context, tx authzuow.TxRepositories) error) error {
	return fn(ctx, u.tx)
}

type assignmentEventStagerStub struct {
	events []event.DomainEvent
}

func (s *assignmentEventStagerStub) Stage(_ context.Context, events ...event.DomainEvent) error {
	s.events = append(s.events, events...)
	return nil
}

type assignmentRepoStub struct {
	createCalls int
	nextID      uint64
	created     []*assignmentDomain.Assignment
	findByID    map[uint64]*assignmentDomain.Assignment
}

func (r *assignmentRepoStub) Create(_ context.Context, a *assignmentDomain.Assignment) error {
	r.createCalls++
	if r.nextID == 0 {
		r.nextID = 1
	}
	a.ID = assignmentDomain.NewAssignmentID(r.nextID)
	r.nextID++
	r.created = append(r.created, a)
	if r.findByID == nil {
		r.findByID = make(map[uint64]*assignmentDomain.Assignment)
	}
	r.findByID[a.ID.Uint64()] = a
	return nil
}

func (r *assignmentRepoStub) Delete(_ context.Context, id assignmentDomain.AssignmentID) error {
	delete(r.findByID, id.Uint64())
	return nil
}

func (r *assignmentRepoStub) DeleteBySubjectAndRole(_ context.Context, _ assignmentDomain.SubjectType, _ string, _ uint64, _ string) error {
	return nil
}

func (r *assignmentRepoStub) FindByID(_ context.Context, id assignmentDomain.AssignmentID) (*assignmentDomain.Assignment, error) {
	return r.findByID[id.Uint64()], nil
}

func (r *assignmentRepoStub) ListBySubject(_ context.Context, _ assignmentDomain.SubjectType, _ string, _ string) ([]*assignmentDomain.Assignment, error) {
	return nil, nil
}

func (r *assignmentRepoStub) ListByRole(_ context.Context, _ uint64, _ string) ([]*assignmentDomain.Assignment, error) {
	return nil, nil
}

type assignmentRoleRepoStub struct {
	role *roleDomain.Role
}

func (r *assignmentRoleRepoStub) Create(context.Context, *roleDomain.Role) error { return nil }
func (r *assignmentRoleRepoStub) Update(context.Context, *roleDomain.Role) error { return nil }
func (r *assignmentRoleRepoStub) Delete(context.Context, meta.ID) error          { return nil }
func (r *assignmentRoleRepoStub) FindByID(context.Context, meta.ID) (*roleDomain.Role, error) {
	return r.role, nil
}
func (r *assignmentRoleRepoStub) FindByName(context.Context, string, string) (*roleDomain.Role, error) {
	return r.role, nil
}
func (r *assignmentRoleRepoStub) List(context.Context, string, int, int) ([]*roleDomain.Role, int64, error) {
	return nil, 0, nil
}

type policyVersionRepoStub struct {
	currentVersion int64
	incrementCalls int
}

func (r *policyVersionRepoStub) GetOrCreate(_ context.Context, tenantID string) (*policyDomain.PolicyVersion, error) {
	version := policyDomain.NewPolicyVersion(tenantID, r.currentVersion)
	return &version, nil
}

func (r *policyVersionRepoStub) Increment(_ context.Context, tenantID, changedBy, reason string) (*policyDomain.PolicyVersion, error) {
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

func (r *policyVersionRepoStub) GetCurrent(_ context.Context, tenantID string) (*policyDomain.PolicyVersion, error) {
	version := policyDomain.NewPolicyVersion(tenantID, r.currentVersion)
	return &version, nil
}

type ruleStoreStub struct {
	groupingAdds []policyDomain.GroupingRule
}

func (r *ruleStoreStub) AddPolicy(context.Context, ...policyDomain.PolicyRule) error { return nil }
func (r *ruleStoreStub) RemovePolicy(context.Context, ...policyDomain.PolicyRule) error {
	return nil
}
func (r *ruleStoreStub) AddGroupingPolicy(_ context.Context, rules ...policyDomain.GroupingRule) error {
	r.groupingAdds = append(r.groupingAdds, rules...)
	return nil
}
func (r *ruleStoreStub) RemoveGroupingPolicy(context.Context, ...policyDomain.GroupingRule) error {
	return nil
}

type casbinAdapterStub struct {
	loadErr   error
	loadCalls int
}

func (s *casbinAdapterStub) AddPolicy(context.Context, ...policyDomain.PolicyRule) error { return nil }
func (s *casbinAdapterStub) RemovePolicy(context.Context, ...policyDomain.PolicyRule) error {
	return nil
}
func (s *casbinAdapterStub) AddGroupingPolicy(context.Context, ...policyDomain.GroupingRule) error {
	return nil
}
func (s *casbinAdapterStub) RemoveGroupingPolicy(context.Context, ...policyDomain.GroupingRule) error {
	return nil
}
func (s *casbinAdapterStub) GetPoliciesByRole(context.Context, string, string) ([]policyDomain.PolicyRule, error) {
	return nil, nil
}
func (s *casbinAdapterStub) GetGroupingsBySubject(context.Context, string, string) ([]policyDomain.GroupingRule, error) {
	return nil, nil
}
func (s *casbinAdapterStub) LoadPolicy(context.Context) error {
	s.loadCalls++
	return s.loadErr
}
func (s *casbinAdapterStub) Enforce(context.Context, string, string, string, string) (bool, error) {
	return false, nil
}
func (s *casbinAdapterStub) GetRolesForUser(context.Context, string, string) ([]string, error) {
	return nil, nil
}
func (s *casbinAdapterStub) GetImplicitRolesForUser(context.Context, string, string) ([]string, error) {
	return nil, nil
}
func (s *casbinAdapterStub) GetImplicitPermissionsForUser(context.Context, string, string) ([]policyDomain.PolicyRule, error) {
	return nil, nil
}
func (s *casbinAdapterStub) InvalidateCache() {}
