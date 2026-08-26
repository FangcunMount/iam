package authz_test

import (
	"context"
	"errors"
	"sort"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	rolebindingApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/rolebinding"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	bindingDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	policyRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/policy"
	roleRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	bindingRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/rolebinding"
	authzUOW "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v3/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/FangcunMount/iam/v3/pkg/event"
	"github.com/stretchr/testify/require"
)

func TestReplaceManagedAssignmentsIsAtomicAndPreservesUnmanagedRoles(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&roleRepo.RolePO{}, &bindingRepo.BindingPO{}, &policyRepo.PolicyVersionPO{}))

	ctx := context.Background()
	roles := roleRepo.NewRoleRepository(db)
	bindings := bindingRepo.NewBindingRepository(db)
	roleByName := seedRoles(t, ctx, roles, "tenant_admin", "qs:staff", "qs:evaluator", "qs:content_manager")
	userID := meta.FromUint64(100)
	seedBinding(t, ctx, bindings, userID, roleByName["tenant_admin"].ID)
	seedBinding(t, ctx, bindings, userID, roleByName["qs:evaluator"].ID)

	stager := &eventStager{}
	uow := authzUOW.NewUnitOfWork(db, existingUserResolver{}, stager)
	validator := bindingDomain.NewValidator(bindings, roles, existingUserResolver{})
	service := rolebindingApp.NewCommandService(validator, roles, uow, nil)
	sub, err := subject.NewUserRef(userID)
	require.NoError(t, err)
	managed := []string{"qs:staff", "qs:evaluator", "qs:content_manager"}

	cmd, err := rolebindingApp.NewReplaceManagedAssignmentsCommand(
		sub, "fangcun", []string{"qs:staff", "qs:content_manager"}, managed,
		"user:200", "staff role update",
	)
	require.NoError(t, err)
	result, err := service.ReplaceManagedAssignments(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result.Changed)
	require.EqualValues(t, 1, result.PolicyVersion)
	require.Equal(t, []string{"qs:content_manager", "qs:staff"}, result.DirectRoles)
	require.Equal(t, []string{"qs:content_manager", "qs:staff", "tenant_admin"}, assignedRoleNames(t, ctx, bindings, roles, userID))
	require.Len(t, stager.events, 1)

	result, err = service.ReplaceManagedAssignments(ctx, cmd)
	require.NoError(t, err)
	require.False(t, result.Changed)
	require.EqualValues(t, 1, result.PolicyVersion)
	require.Len(t, stager.events, 1, "idempotent replacement must not emit another version event")

	stager.err = errors.New("outbox unavailable")
	rollbackCmd, err := rolebindingApp.NewReplaceManagedAssignmentsCommand(
		sub, "fangcun", []string{"qs:evaluator"}, managed,
		"user:200", "rollback proof",
	)
	require.NoError(t, err)
	_, err = service.ReplaceManagedAssignments(ctx, rollbackCmd)
	require.ErrorContains(t, err, "outbox unavailable")
	require.Equal(t, []string{"qs:content_manager", "qs:staff", "tenant_admin"}, assignedRoleNames(t, ctx, bindings, roles, userID))
	current, err := policyRepo.NewPolicyVersionRepository(db).GetCurrent(ctx, "fangcun")
	require.NoError(t, err)
	require.NotNil(t, current)
	require.EqualValues(t, 1, current.Version, "failed replacement must roll back the policy version")
}

func TestReplaceManagedAssignmentsCommandRejectsDuplicateAndUnmanagedTargets(t *testing.T) {
	sub, err := subject.NewUserRef(meta.FromUint64(100))
	require.NoError(t, err)
	_, err = rolebindingApp.NewReplaceManagedAssignmentsCommand(
		sub, "fangcun", []string{"qs:staff", "qs:staff"}, []string{"qs:staff"}, "user:200", "",
	)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
	_, err = rolebindingApp.NewReplaceManagedAssignmentsCommand(
		sub, "fangcun", []string{"tenant_admin"}, []string{"qs:staff"}, "user:200", "",
	)
	require.True(t, perrors.IsCode(err, code.ErrPermissionDenied))
}

type existingUserResolver struct{}

func (existingUserResolver) ResolveUser(context.Context, meta.ID) error { return nil }

type eventStager struct {
	events []event.DomainEvent
	err    error
}

func (s *eventStager) Stage(_ context.Context, events ...event.DomainEvent) error {
	if s.err != nil {
		return s.err
	}
	s.events = append(s.events, events...)
	return nil
}

func seedRoles(t *testing.T, ctx context.Context, repo roleDomain.Repository, names ...string) map[string]*roleDomain.Role {
	t.Helper()
	result := make(map[string]*roleDomain.Role, len(names))
	for _, name := range names {
		role, err := roleDomain.NewRole(name, name, "fangcun")
		require.NoError(t, err)
		require.NoError(t, repo.Create(ctx, &role))
		copyRole := role
		result[name] = &copyRole
	}
	return result
}

func seedBinding(t *testing.T, ctx context.Context, repo bindingDomain.Repository, subjectID, roleID meta.ID) {
	t.Helper()
	binding, err := bindingDomain.NewBinding(
		bindingDomain.SubjectTypeUser, subjectID, roleID, "fangcun", bindingDomain.WithGrantedBy("seed"),
	)
	require.NoError(t, err)
	require.NoError(t, repo.Create(ctx, &binding))
}

func assignedRoleNames(
	t *testing.T,
	ctx context.Context,
	bindings bindingDomain.Repository,
	roles roleDomain.Repository,
	subjectID meta.ID,
) []string {
	t.Helper()
	rows, err := bindings.ListBySubject(ctx, bindingDomain.SubjectTypeUser, subjectID, "fangcun")
	require.NoError(t, err)
	names := make([]string, 0, len(rows))
	for _, binding := range rows {
		role, err := roles.FindByID(ctx, binding.RoleID)
		require.NoError(t, err)
		names = append(names, role.NameString())
	}
	sort.Strings(names)
	return names
}

var _ event.Stager = (*eventStager)(nil)
