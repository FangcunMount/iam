package rolebinding

import (
	"context"
	"errors"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authzuow "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/uow"
	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	policyDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	roleDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	bindingDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/v2/internal/apiserver/eventing"
	"github.com/FangcunMount/iam/v2/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/FangcunMount/iam/v2/pkg/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBindingCommandServiceGrant_RejectsMissingUserWithoutWrites(t *testing.T) {
	roleRepo := &bindingRoleRepoStub{
		role: &roleDomain.Role{
			ID:       meta.FromUint64(10),
			Name:     "iam:admin",
			TenantID: "tenant-a",
		},
	}
	bindingRepo := &bindingRepoStub{}
	userRepo := testhelpers.NewUserRepoStub()
	versionRepo := &policyVersionRepoStub{}
	ruleStore := &ruleStoreStub{}
	runtime := &casbinAdapterStub{}
	stager := &bindingEventStagerStub{}

	validator := bindingDomain.NewValidator(bindingRepo, roleRepo, userRepo)
	service := NewCommandService(
		validator,
		roleRepo,
		&uowStub{tx: authzuow.TxRepositories{
			Bindings:           bindingRepo,
			Roles:              roleRepo,
			Users:              userRepo,
			PolicyVersions:     versionRepo,
			AuthorizationFacts: ruleStore,
			Events:             stager,
		}},
		runtime,
	)

	_, err := service.Grant(context.Background(), GrantCommand{
		SubjectType: bindingDomain.SubjectTypeUser,
		SubjectID:   "123",
		RoleID:      10,
		TenantID:    "tenant-a",
		GrantedBy:   "1",
	})
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrUserNotFound))
	assert.Equal(t, 0, bindingRepo.createCalls)
	assert.Len(t, ruleStore.groupingAdds, 0)
	assert.Equal(t, 0, versionRepo.incrementCalls)
	assert.Equal(t, 0, runtime.loadCalls)
}

func TestBindingCommandServiceGrant_CommitsFactsWhenRuntimeReloadFails(t *testing.T) {
	roleRepo := &bindingRoleRepoStub{
		role: &roleDomain.Role{
			ID:       meta.FromUint64(10),
			Name:     "iam:admin",
			TenantID: "tenant-a",
		},
	}
	bindingRepo := &bindingRepoStub{}
	userRepo := testhelpers.NewUserRepoStub()
	userRepo.UsersByID[123] = &userDomain.User{ID: meta.FromUint64(123)}
	versionRepo := &policyVersionRepoStub{}
	ruleStore := &ruleStoreStub{}
	runtime := &casbinAdapterStub{loadErr: errors.New("reload failed")}
	stager := &bindingEventStagerStub{}

	validator := bindingDomain.NewValidator(bindingRepo, roleRepo, userRepo)
	service := NewCommandService(
		validator,
		roleRepo,
		&uowStub{tx: authzuow.TxRepositories{
			Bindings:           bindingRepo,
			Roles:              roleRepo,
			Users:              userRepo,
			PolicyVersions:     versionRepo,
			AuthorizationFacts: ruleStore,
			Events:             stager,
		}},
		runtime,
	)

	result, err := service.Grant(context.Background(), GrantCommand{
		SubjectType: bindingDomain.SubjectTypeUser,
		SubjectID:   "123",
		RoleID:      10,
		TenantID:    "tenant-a",
		GrantedBy:   "1",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, uint64(1), result.ID.Uint64())
	assert.Equal(t, 1, bindingRepo.createCalls)
	require.Len(t, ruleStore.groupingAdds, 1)
	assert.Equal(t, authzDomain.SubjectTypeUser, ruleStore.groupingAdds[0].Subject.Type)
	assert.Equal(t, "123", ruleStore.groupingAdds[0].Subject.ID)
	assert.Equal(t, "iam:admin", ruleStore.groupingAdds[0].RoleName)
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

type bindingEventStagerStub struct {
	events []event.DomainEvent
}

func (s *bindingEventStagerStub) Stage(_ context.Context, events ...event.DomainEvent) error {
	s.events = append(s.events, events...)
	return nil
}

type bindingRepoStub struct {
	createCalls int
	nextID      uint64
	created     []*bindingDomain.Binding
	findByID    map[uint64]*bindingDomain.Binding
}

func (r *bindingRepoStub) Create(_ context.Context, a *bindingDomain.Binding) error {
	r.createCalls++
	if r.nextID == 0 {
		r.nextID = 1
	}
	a.ID = bindingDomain.NewBindingID(r.nextID)
	r.nextID++
	r.created = append(r.created, a)
	if r.findByID == nil {
		r.findByID = make(map[uint64]*bindingDomain.Binding)
	}
	r.findByID[a.ID.Uint64()] = a
	return nil
}

func (r *bindingRepoStub) Delete(_ context.Context, id bindingDomain.BindingID) error {
	delete(r.findByID, id.Uint64())
	return nil
}

func (r *bindingRepoStub) DeleteBySubjectAndRole(_ context.Context, _ bindingDomain.SubjectType, _ string, _ uint64, _ string) error {
	return nil
}

func (r *bindingRepoStub) FindByID(_ context.Context, id bindingDomain.BindingID) (*bindingDomain.Binding, error) {
	return r.findByID[id.Uint64()], nil
}

func (r *bindingRepoStub) ListBySubject(_ context.Context, _ bindingDomain.SubjectType, _ string, _ string) ([]*bindingDomain.Binding, error) {
	return nil, nil
}

func (r *bindingRepoStub) ListByRole(_ context.Context, _ uint64, _ string) ([]*bindingDomain.Binding, error) {
	return nil, nil
}

type bindingRoleRepoStub struct {
	role *roleDomain.Role
}

func (r *bindingRoleRepoStub) Create(context.Context, *roleDomain.Role) error { return nil }
func (r *bindingRoleRepoStub) Update(context.Context, *roleDomain.Role) error { return nil }
func (r *bindingRoleRepoStub) Delete(context.Context, meta.ID) error          { return nil }
func (r *bindingRoleRepoStub) FindByID(context.Context, meta.ID) (*roleDomain.Role, error) {
	return r.role, nil
}
func (r *bindingRoleRepoStub) FindByName(context.Context, string, string) (*roleDomain.Role, error) {
	return r.role, nil
}
func (r *bindingRoleRepoStub) List(context.Context, string, int, int) ([]*roleDomain.Role, int64, error) {
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
	groupingAdds []authzDomain.RoleBinding
}

func (r *ruleStoreStub) AddPermission(context.Context, authzDomain.Permission) error { return nil }
func (r *ruleStoreStub) RemovePermission(context.Context, authzDomain.Permission) error {
	return nil
}
func (r *ruleStoreStub) AddRoleBinding(_ context.Context, binding authzDomain.RoleBinding) error {
	r.groupingAdds = append(r.groupingAdds, binding)
	return nil
}
func (r *ruleStoreStub) RemoveRoleBinding(context.Context, authzDomain.RoleBinding) error {
	return nil
}

type casbinAdapterStub struct {
	loadErr   error
	loadCalls int
}

func (s *casbinAdapterStub) LoadPolicy(context.Context) error {
	s.loadCalls++
	return s.loadErr
}
func (s *casbinAdapterStub) InvalidateCache() {}
