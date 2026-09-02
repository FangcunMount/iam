package authz_test

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	assignmentApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignment"
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
	uow := authzUOW.NewUnitOfWork(db, existingUserResolver{}, stager)
	validator := assignmentDomain.NewValidator(roles, existingUserResolver{})
	service := assignmentApp.NewCommandService(validator, roles, uow, nil)

	policyVersions := policyRepo.NewPolicyVersionRepository(db)
	beforeVersion := currentPolicyVersion(t, ctx, policyVersions, tenantID)
	beforeEvents := len(stager.events)

	barrier := make(chan struct{})
	var overlap atomic.Bool
	var wg sync.WaitGroup
	wg.Add(2)
	runReplace := func(target []string, changedBy string) {
		defer wg.Done()
		<-barrier
		overlap.Store(true)
		cmd, cmdErr := assignmentApp.NewReplaceManagedAssignmentsCommand(sub, tenantID, target, managed, changedBy, "concurrent-replace")
		require.NoError(t, cmdErr)
		_, replaceErr := service.ReplaceManagedAssignments(ctx, cmd)
		require.NoError(t, replaceErr)
	}
	go runReplace([]string{"qs:staff"}, "user:concurrent-a")
	go runReplace([]string{"qs:evaluator"}, "user:concurrent-b")
	close(barrier)
	wg.Wait()
	require.True(t, overlap.Load(), "both replace operations must overlap")

	final := assignedTenantRoleNames(t, ctx, assignments, roles, tenantID, userID)
	validTargets := [][]string{{"qs:staff"}, {"qs:evaluator"}}
	require.Contains(t, validTargets, final, "concurrent replace must end in one complete managed target set")

	afterVersion := currentPolicyVersion(t, ctx, policyVersions, tenantID)
	require.GreaterOrEqual(t, afterVersion, beforeVersion+1)
	require.LessOrEqual(t, afterVersion, beforeVersion+2)
	require.GreaterOrEqual(t, len(stager.events), beforeEvents+1)
	require.LessOrEqual(t, len(stager.events), beforeEvents+2)
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
