package assignment_test

import (
	"context"
	"fmt"
	"testing"

	assignmentapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignment"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/testutil"
	assignmentdomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/FangcunMount/iam/v3/pkg/event"
	"github.com/stretchr/testify/require"
)

type recordingResolver struct{ calls int }

func (*recordingResolver) Supports(kind subject.Type) bool { return kind == subject.TypeUser }
func (r *recordingResolver) Resolve(context.Context, subject.Ref, tenant.ID) error {
	r.calls++
	return nil
}

type rejectedEvent struct{}

func (rejectedEvent) Stage(context.Context, ...event.DomainEvent) error {
	return fmt.Errorf("outbox unavailable")
}
func TestIDAndNameGrantShareResolverTransactionAndRollback(t *testing.T) {
	resolver := &recordingResolver{}
	fixture := testutil.NewAssignmentFixture(t, resolver)
	ctx := context.Background()
	roles := fixture.Roles
	r, err := role.NewRole("reader", "Reader", "a")
	require.NoError(t, err)
	require.NoError(t, roles.Create(ctx, &r))
	validator := assignmentdomain.NewValidatorWithSubjectResolver(roles, resolver)
	service := assignmentapp.NewCommandService(validator, roles, fixture.UnitOfWork, nil)
	cmd, err := assignmentapp.NewGrantCommand(assignmentdomain.SubjectTypeUser, meta.ID(1), r.ID, "a", "seed")
	require.NoError(t, err)
	granted, err := service.Grant(ctx, cmd)
	require.NoError(t, err)
	require.Equal(t, r.ID, granted.RoleID)
	sub, err := subject.NewUserRef(meta.ID(2))
	require.NoError(t, err)
	version, err := service.GrantByRoleName(ctx, assignmentapp.GrantByRoleNameCommand{Subject: sub, TenantID: "a", RoleName: r.Name.String(), GrantedBy: "seed"})
	require.NoError(t, err)
	require.EqualValues(t, 2, version)
	require.Equal(t, 2, resolver.calls)
	assignments, err := fixture.Assignments.ListByRole(ctx, r.ID, "a")
	require.NoError(t, err)
	require.Len(t, assignments, 2)
	for _, a := range assignments {
		require.Equal(t, r.ID, a.RoleID)
		require.Equal(t, tenant.ID("a"), a.TenantID)
	}
	require.EqualValues(t, 2, fixture.OutboxCount(t))
	failed := assignmentapp.NewCommandService(validator, roles, fixture.WithEventStager(rejectedEvent{}), nil)
	cmd.SubjectID = meta.ID(3)
	_, err = failed.Grant(ctx, cmd)
	require.Error(t, err)
	sub, err = subject.NewUserRef(meta.ID(4))
	require.NoError(t, err)
	_, err = failed.GrantByRoleName(ctx, assignmentapp.GrantByRoleNameCommand{Subject: sub, TenantID: "a", RoleName: r.Name.String(), GrantedBy: "seed"})
	require.Error(t, err)
	assignments, err = fixture.Assignments.ListByRole(ctx, r.ID, "a")
	require.NoError(t, err)
	require.Len(t, assignments, 2)
	require.EqualValues(t, 2, fixture.OutboxCount(t))
}
