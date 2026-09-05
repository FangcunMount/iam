package testutil

import (
	"testing"

	authzuow "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/uow"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	assignmentrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/assignment"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/eventoutbox"
	rolerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	mysqluow "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v3/internal/apiserver/testfixtures/authzdb"
	"github.com/FangcunMount/iam/v3/pkg/event"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type AssignmentFixture struct {
	Roles       role.Repository
	Assignments assignment.Repository
	UnitOfWork  authzuow.UnitOfWork
	db          *gorm.DB
	resolver    assignment.SubjectResolver
}

func NewAssignmentFixture(t *testing.T, resolver assignment.SubjectResolver) *AssignmentFixture {
	t.Helper()
	db := authzdb.Open(t, false)
	return &AssignmentFixture{
		Roles: rolerepo.NewRoleRepository(db), Assignments: assignmentrepo.NewRepository(db),
		UnitOfWork: mysqluow.NewUnitOfWork(db, resolver, authzdb.Stager(t, db)),
		db:         db, resolver: resolver,
	}
}

func (f *AssignmentFixture) WithEventStager(stager event.Stager) authzuow.UnitOfWork {
	return mysqluow.NewUnitOfWork(f.db, f.resolver, stager)
}

func (f *AssignmentFixture) OutboxCount(t *testing.T) int64 {
	t.Helper()
	var count int64
	require.NoError(t, f.db.Model(&eventoutbox.OutboxPO{}).Count(&count).Error)
	return count
}
