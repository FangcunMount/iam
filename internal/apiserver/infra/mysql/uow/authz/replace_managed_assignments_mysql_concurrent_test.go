package authz_test

import (
	"context"
	"fmt"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/subjectresolver"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	assignmentApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignment"
	authzAppUOW "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/uow"
	assignmentDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	policyDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/policy"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	assignmentRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/assignment"
	policyRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/policy"
	roleRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	authzUOW "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestReplaceManagedAssignmentsMySQLConcurrentLinearization(t *testing.T) {
	host := os.Getenv("MYSQL_HOST")
	if host == "" {
		t.Skip("MYSQL_HOST is not configured")
	}
	dsn := mysqlDSN(host)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&roleRepo.RolePO{},
		&assignmentRepo.AssignmentPO{},
		&policyRepo.PolicyVersionPO{},
	))

	tenantID := fmt.Sprintf("replace-concurrent-%d", time.Now().UnixNano())
	userID := meta.FromUint64(uint64(time.Now().UnixNano()%900_000_000 + 100_000_000))
	t.Cleanup(func() {
		ctx := context.Background()
		require.NoError(t, db.WithContext(ctx).Unscoped().Where("tenant_id = ?", tenantID).Delete(&assignmentRepo.AssignmentPO{}).Error)
		require.NoError(t, db.WithContext(ctx).Unscoped().Where("tenant_id = ?", tenantID).Delete(&roleRepo.RolePO{}).Error)
		require.NoError(t, db.WithContext(ctx).Unscoped().Where("tenant_id = ?", tenantID).Delete(&policyRepo.PolicyVersionPO{}).Error)
	})

	ctx := context.Background()
	roles := roleRepo.NewRoleRepository(db)
	assignments := assignmentRepo.NewRepository(db)
	roleByName := seedTenantRoles(t, ctx, roles, tenantID, "qs:staff", "qs:evaluator")
	seedTenantAssignment(t, ctx, assignments, tenantID, userID, roleByName["qs:staff"].ID)
	seedTenantAssignment(t, ctx, assignments, tenantID, userID, roleByName["qs:evaluator"].ID)

	sub, err := subject.NewUserRef(userID)
	require.NoError(t, err)
	managed := []string{"qs:staff", "qs:evaluator"}
	stager := &eventStager{}
	barrier := newRoleReadBarrier()
	defer barrier.Release()
	uow := &roleReadBarrierUnitOfWork{
		delegate: authzUOW.NewUnitOfWork(db, subjectresolver.NewUserSubjectResolver(existingUserResolver{}), stager),
		barrier:  barrier,
	}
	validator := assignmentDomain.NewValidator(roles, subjectresolver.NewUserSubjectResolver(existingUserResolver{}))
	service := assignmentApp.NewCommandService(validator, roles, uow, nil)

	policyVersions := policyRepo.NewPolicyVersionRepository(db)
	beforeVersion := currentPolicyVersion(t, ctx, policyVersions, tenantID)
	beforeEvents := stager.Count()

	commands := make([]assignmentApp.ReplaceManagedAssignmentsCommand, 0, 2)
	for _, input := range []struct {
		target    []string
		changedBy string
	}{
		{target: []string{"qs:staff"}, changedBy: "user:concurrent-a"},
		{target: []string{"qs:evaluator"}, changedBy: "user:concurrent-b"},
	} {
		cmd, cmdErr := assignmentApp.NewReplaceManagedAssignmentsCommand(
			sub, tenantID, input.target, managed, input.changedBy, "concurrent-replace",
		)
		require.NoError(t, cmdErr)
		commands = append(commands, cmd)
	}

	type replaceResult struct {
		result assignmentApp.ReplaceManagedAssignmentsResult
		err    error
	}
	results := make(chan replaceResult, len(commands))
	for _, cmd := range commands {
		cmd := cmd
		go func() {
			result, replaceErr := service.ReplaceManagedAssignments(ctx, cmd)
			results <- replaceResult{result: result, err: replaceErr}
		}()
	}
	for range commands {
		select {
		case <-barrier.arrived:
		case result := <-results:
			require.NoError(t, result.err)
			t.Fatal("replace returned before both transactions reached the role-read barrier")
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for concurrent replace transactions")
		}
	}
	barrier.Release()
	for range commands {
		result := <-results
		require.NoError(t, result.err)
		require.True(t, result.result.Changed)
	}

	final := assignedTenantRoleNames(t, ctx, assignments, roles, tenantID, userID)
	validTargets := [][]string{{"qs:staff"}, {"qs:evaluator"}}
	require.Contains(t, validTargets, final, "concurrent replace must end in one complete managed target set")

	afterVersion := currentPolicyVersion(t, ctx, policyVersions, tenantID)
	require.Equal(t, beforeVersion+2, afterVersion)
	require.Equal(t, beforeEvents+2, stager.Count())
}

type roleReadBarrier struct {
	arrived     chan struct{}
	release     chan struct{}
	releaseOnce sync.Once
}

func newRoleReadBarrier() *roleReadBarrier {
	return &roleReadBarrier{arrived: make(chan struct{}, 2), release: make(chan struct{})}
}

func (b *roleReadBarrier) ArriveAndWait() {
	b.arrived <- struct{}{}
	<-b.release
}

func (b *roleReadBarrier) Release() {
	b.releaseOnce.Do(func() { close(b.release) })
}

type roleReadBarrierUnitOfWork struct {
	delegate authzAppUOW.UnitOfWork
	barrier  *roleReadBarrier
}

func (u *roleReadBarrierUnitOfWork) WithinTx(
	ctx context.Context,
	fn func(context.Context, authzAppUOW.TxRepositories) error,
) error {
	return u.delegate.WithinTx(ctx, func(txCtx context.Context, tx authzAppUOW.TxRepositories) error {
		tx.Roles = &roleReadBarrierRepository{Repository: tx.Roles, barrier: u.barrier}
		return fn(txCtx, tx)
	})
}

type roleReadBarrierRepository struct {
	roleDomain.Repository
	barrier *roleReadBarrier
	once    sync.Once
}

func (r *roleReadBarrierRepository) FindByName(ctx context.Context, tenantID, name string) (*roleDomain.Role, error) {
	role, err := r.Repository.FindByName(ctx, tenantID, name)
	if err == nil {
		r.once.Do(r.barrier.ArriveAndWait)
	}
	return role, err
}

func mysqlDSN(host string) string {
	if dsn := os.Getenv("MYSQL_DSN"); dsn != "" {
		return dsn
	}
	user := os.Getenv("MYSQL_USER")
	if user == "" {
		user = "root"
	}
	password := os.Getenv("MYSQL_PASSWORD")
	port := os.Getenv("MYSQL_PORT")
	if port == "" {
		port = "3306"
	}
	database := os.Getenv("MYSQL_DATABASE")
	if database == "" {
		database = "iam_test"
	}
	return user + ":" + password + "@tcp(" + host + ":" + port + ")/" + database + "?charset=utf8mb4&parseTime=True&loc=Local"
}

func seedTenantRoles(t *testing.T, ctx context.Context, repo roleDomain.Repository, tenantID string, names ...string) map[string]*roleDomain.Role {
	t.Helper()
	result := make(map[string]*roleDomain.Role, len(names))
	for _, name := range names {
		role, err := roleDomain.NewRole(name, name, tenantID)
		require.NoError(t, err)
		require.NoError(t, repo.Create(ctx, &role))
		copyRole := role
		result[name] = &copyRole
	}
	return result
}

func seedTenantAssignment(t *testing.T, ctx context.Context, repo assignmentDomain.Repository, tenantID string, subjectID, roleID meta.ID) {
	t.Helper()
	assignment, err := assignmentDomain.NewAssignment(
		assignmentDomain.SubjectTypeUser, subjectID, roleID, tenantID, assignmentDomain.WithGrantedBy("seed"),
	)
	require.NoError(t, err)
	require.NoError(t, repo.Create(ctx, &assignment))
}

func assignedTenantRoleNames(
	t *testing.T,
	ctx context.Context,
	assignments assignmentDomain.Repository,
	roles roleDomain.Repository,
	tenantID string,
	subjectID meta.ID,
) []string {
	t.Helper()
	rows, err := assignments.ListBySubject(ctx, assignmentDomain.SubjectTypeUser, subjectID, tenantID)
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

func currentPolicyVersion(t *testing.T, ctx context.Context, repo policyDomain.Repository, tenantID string) int64 {
	t.Helper()
	current, err := repo.GetCurrent(ctx, tenantID)
	require.NoError(t, err)
	if current == nil {
		return 0
	}
	return current.Version
}
