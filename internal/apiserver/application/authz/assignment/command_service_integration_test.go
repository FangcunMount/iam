package assignment_test

import (
	"context"
	"fmt"
	"testing"

	assignmentapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignment"
	assignmentdomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	assignmentrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/assignment"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/eventoutbox"
	rolerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	authzuow "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v3/internal/apiserver/testfixtures/authzdb"
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
	db := authzdb.Open(t, false)
	ctx := context.Background()
	roles := rolerepo.NewRoleRepository(db)
	r, err := role.NewRole("reader", "Reader", "a")
	require.NoError(t, err)
	require.NoError(t, roles.Create(ctx, &r))
	resolver := &recordingResolver{}
	validator := assignmentdomain.NewValidatorWithSubjectResolver(roles, resolver)
	uow := authzuow.NewUnitOfWork(db, resolver, authzdb.Stager(t, db))
	service := assignmentapp.NewCommandService(validator, roles, uow, nil)
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
	assignments, err := assignmentrepo.NewRepository(db).ListByRole(ctx, r.ID, "a")
	require.NoError(t, err)
	require.Len(t, assignments, 2)
	for _, a := range assignments {
		require.Equal(t, r.ID, a.RoleID)
		require.Equal(t, tenant.ID("a"), a.TenantID)
	}
	var events int64
	require.NoError(t, db.Model(&eventoutbox.OutboxPO{}).Count(&events).Error)
	require.EqualValues(t, 2, events)
	failed := assignmentapp.NewCommandService(validator, roles, authzuow.NewUnitOfWork(db, resolver, rejectedEvent{}), nil)
	cmd.SubjectID = meta.ID(3)
	_, err = failed.Grant(ctx, cmd)
	require.Error(t, err)
	sub, err = subject.NewUserRef(meta.ID(4))
	require.NoError(t, err)
	_, err = failed.GrantByRoleName(ctx, assignmentapp.GrantByRoleNameCommand{Subject: sub, TenantID: "a", RoleName: r.Name.String(), GrantedBy: "seed"})
	require.Error(t, err)
	assignments, err = assignmentrepo.NewRepository(db).ListByRole(ctx, r.ID, "a")
	require.NoError(t, err)
	require.Len(t, assignments, 2)
	require.NoError(t, db.Model(&eventoutbox.OutboxPO{}).Count(&events).Error)
	require.EqualValues(t, 2, events)
}
