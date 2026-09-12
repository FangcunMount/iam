package authz_test

import (
	"context"
	"errors"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"
	"sort"
	"sync"
	"testing"

	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/management"

	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/authz/subjectresolver"

	assignmentApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/assignment"
	authzAppUOW "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/uow"
	assignmentDomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/assignment"
	roleDomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/subject"
	assignmentRepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/assignment"
	policyRepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/policy"
	roleRepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/role"
	authzUOW "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v5/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/stretchr/testify/require"
)

func TestReplaceManagedAssignmentsIsAtomicAndPreservesUnmanagedRoles(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&roleRepo.RolePO{}, &assignmentRepo.AssignmentPO{}, &policyRepo.PolicyVersionPO{}))

	ctx := management.WithAuthenticatedService(context.Background(), "admin")
	roles := roleRepo.NewRoleRepository(db)
	assignments := assignmentRepo.NewRepository(db)
	roleByName := seedRoles(t, ctx, roles, "iam_admin", "qs:staff", "qs:evaluator", "qs:content_manager")
	userID := meta.FromUint64(100)
	seedAssignment(t, ctx, assignments, userID, roleByName["iam_admin"].ID)
	seedAssignment(t, ctx, assignments, userID, roleByName["qs:evaluator"].ID)

	stager := &eventStager{}
	uow := &lockingAssignmentReadUnitOfWork{
		delegate: authzUOW.NewUnitOfWork(db, subjectresolver.NewUserSubjectResolver(existingUserResolver{}), stager),
	}
	validator := assignmentDomain.NewValidator(roles, subjectresolver.NewUserSubjectResolver(existingUserResolver{}))
	service := assignmentApp.NewCommandService(validator, roles, uow, nil, management.NewGuard(nil))
	sub, err := subject.NewUserRef(userID)
	require.NoError(t, err)
	managed := []string{"qs:staff", "qs:evaluator", "qs:content_manager"}

	cmd, err := assignmentApp.NewReplaceManagedAssignmentsCommand(
		sub, []string{"qs:staff", "qs:content_manager"}, managed,
		"user:200", "staff role update",
	)
	require.NoError(t, err)
	result, err := service.ReplaceManagedAssignments(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result.Changed)
	require.EqualValues(t, 2, result.PolicyVersion)
	require.Equal(t, []string{"qs:content_manager", "qs:staff"}, result.DirectRoles)
	require.Equal(t, []string{"iam_admin", "qs:content_manager", "qs:staff"}, assignedRoleNames(t, ctx, assignments, roles, userID))
	require.Equal(t, 1, stager.Count())
	require.True(t, uow.usedLockingRead, "replacement must read current assignments with a transaction lock")

	result, err = service.ReplaceManagedAssignments(ctx, cmd)
	require.NoError(t, err)
	require.False(t, result.Changed)
	require.EqualValues(t, 2, result.PolicyVersion)
	require.Equal(t, 1, stager.Count(), "idempotent replacement must not emit another version event")

	stager.SetError(errors.New("outbox unavailable"))
	rollbackCmd, err := assignmentApp.NewReplaceManagedAssignmentsCommand(
		sub, []string{"qs:evaluator"}, managed,
		"user:200", "rollback proof",
	)
	require.NoError(t, err)
	_, err = service.ReplaceManagedAssignments(ctx, rollbackCmd)
	require.ErrorContains(t, err, "outbox unavailable")
	require.Equal(t, []string{"iam_admin", "qs:content_manager", "qs:staff"}, assignedRoleNames(t, ctx, assignments, roles, userID))
	current, err := policyRepo.NewPolicyVersionRepository(db).GetCurrent(ctx)
	require.NoError(t, err)
	require.NotNil(t, current)
	require.EqualValues(t, 2, current.Version, "failed replacement must roll back the policy version")
}

type existingUserResolver struct{}

func (existingUserResolver) ResolveUser(context.Context, meta.ID) error { return nil }

type eventStager struct {
	mu     sync.Mutex
	events []event.DomainEvent
	err    error
}

func (s *eventStager) Stage(_ context.Context, events ...event.DomainEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.events = append(s.events, events...)
	return nil
}

func (s *eventStager) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

func (s *eventStager) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

type lockingAssignmentReadUnitOfWork struct {
	delegate        authzAppUOW.UnitOfWork
	usedLockingRead bool
}

func (u *lockingAssignmentReadUnitOfWork) WithinTx(
	ctx context.Context,
	fn func(context.Context, authzAppUOW.TxRepositories) error,
) error {
	return u.delegate.WithinTx(ctx, func(txCtx context.Context, tx authzAppUOW.TxRepositories) error {
		tx.Assignments = &lockingAssignmentReadRepository{
			Repository: tx.Assignments,
			onLockingRead: func() {
				u.usedLockingRead = true
			},
		}
		return fn(txCtx, tx)
	})
}

type lockingAssignmentReadRepository struct {
	assignmentDomain.Repository
	onLockingRead func()
}

func (r *lockingAssignmentReadRepository) ListBySubject(
	context.Context,
	assignmentDomain.SubjectType,
	meta.ID,
) ([]*assignmentDomain.Assignment, error) {
	return nil, errors.New("managed replacement must use ListBySubjectForUpdate")
}

func (r *lockingAssignmentReadRepository) ListBySubjectForUpdate(
	ctx context.Context,
	subjectType assignmentDomain.SubjectType,
	subjectID meta.ID,

) ([]*assignmentDomain.Assignment, error) {
	r.onLockingRead()
	return r.Repository.ListBySubjectForUpdate(ctx, subjectType, subjectID)
}

func seedRoles(t *testing.T, ctx context.Context, repo roleDomain.Repository, names ...string) map[string]*roleDomain.Role {
	t.Helper()
	result := make(map[string]*roleDomain.Role, len(names))
	for _, name := range names {
		role, err := roleDomain.NewRole(name, name)
		require.NoError(t, err)
		require.NoError(t, repo.Create(ctx, &role))
		copyRole := role
		result[name] = &copyRole
	}
	return result
}

func seedAssignment(t *testing.T, ctx context.Context, repo assignmentDomain.Repository, subjectID, roleID meta.ID) {
	t.Helper()
	assignment, err := assignmentDomain.NewAssignment(
		assignmentDomain.SubjectTypeUser, subjectID, roleID, assignmentDomain.WithGrantedBy("seed"),
	)
	require.NoError(t, err)
	require.NoError(t, repo.Create(ctx, &assignment))
}

func assignedRoleNames(
	t *testing.T,
	ctx context.Context,
	assignments assignmentDomain.Repository,
	roles roleDomain.Repository,
	subjectID meta.ID,
) []string {
	t.Helper()
	rows, err := assignments.ListBySubject(ctx, assignmentDomain.SubjectTypeUser, subjectID)
	require.NoError(t, err)
	names := make([]string, 0, len(rows))
	for _, assignment := range rows {
		role, err := roles.FindByID(ctx, assignment.RoleID)
		require.NoError(t, err)
		names = append(names, role.NameString())
	}
	sort.Strings(names)
	return names
}

var _ event.Stager = (*eventStager)(nil)

func TestScopedReplacementPersistsRangeAndDoesNotTouchOtherCompany(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&roleRepo.RolePO{}, &assignmentRepo.AssignmentPO{}, &policyRepo.PolicyVersionPO{}))
	ctx := management.WithAuthenticatedService(context.Background(), "admin")
	roles := roleRepo.NewRoleRepository(db)
	assignments := assignmentRepo.NewRepository(db)
	dictionary := seedRoles(t, ctx, roles, "qs:assessment_operator")
	role := dictionary["qs:assessment_operator"]
	stager := &eventStager{}
	uow := authzUOW.NewUnitOfWork(db, subjectresolver.NewUserSubjectResolver(existingUserResolver{}), stager)
	validator := assignmentDomain.NewValidator(roles, subjectresolver.NewUserSubjectResolver(existingUserResolver{}))
	service := assignmentApp.NewCommandService(validator, roles, uow, nil, management.NewGuard(nil))
	sub, err := subject.NewUserRef(100)
	require.NoError(t, err)
	value, err := scope.New(1, scope.Stores, []meta.ID{10})
	require.NoError(t, err)
	other, err := scope.New(2, scope.Stores, []meta.ID{20})
	require.NoError(t, err)
	existing, err := assignmentDomain.NewAssignment(assignmentDomain.SubjectTypeUser, 100, role.ID, assignmentDomain.WithGrantedBy("admin"), assignmentDomain.WithScope(other))
	require.NoError(t, err)
	require.NoError(t, assignments.Create(ctx, &existing))
	cmd := assignmentApp.ReplaceManagedAssignmentsCommand{Subject: sub, OrgID: 1, ManagedRoleNames: []string{role.Name.String()}, ScopedRoles: []assignmentDomain.ScopedRoleGrant{{RoleName: role.Name, Scope: value}}, ChangedBy: "user:200"}
	result, err := service.ReplaceManagedAssignments(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result.Changed)
	require.Equal(t, 1, stager.Count())
	repeated, err := service.ReplaceManagedAssignments(ctx, cmd)
	require.NoError(t, err)
	require.False(t, repeated.Changed)
	require.Equal(t, result.PolicyVersion, repeated.PolicyVersion)
	require.Equal(t, 1, stager.Count())
	rows, err := assignments.ListBySubject(ctx, assignmentDomain.SubjectTypeUser, 100)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	restored, err := assignments.FindByID(ctx, existing.ID)
	require.NoError(t, err)
	got, ok := restored.Scope()
	require.True(t, ok)
	require.True(t, got.ContainsStore(2, 20))
	cmd.ExpectedPolicyVersion = result.PolicyVersion + 1
	stale := cmd
	stale.ScopedRoles = nil
	_, err = service.ReplaceManagedAssignments(ctx, stale)
	require.Error(t, err)
	require.Equal(t, 1, stager.Count(), "stale edit must not stage a policy event")
	unchanged, err := assignments.ListBySubject(ctx, assignmentDomain.SubjectTypeUser, 100)
	require.NoError(t, err)
	require.Len(t, unchanged, 2)
	cmd.ExpectedPolicyVersion = result.PolicyVersion
	cmd.ScopedRoles = nil
	_, err = service.ReplaceManagedAssignments(ctx, cmd)
	require.NoError(t, err)
	rows, err = assignments.ListBySubject(ctx, assignmentDomain.SubjectTypeUser, 100)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, existing.ID, rows[0].ID)
}
