package assignment

import (
	"context"
	domain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v5/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestScopeMappingRoundTrip(t *testing.T) {
	for _, kind := range []scope.Kind{scope.Stores, scope.AllStores} {
		t.Run(string(kind), func(t *testing.T) {
			var ids []meta.ID
			if kind == scope.Stores {
				ids = []meta.ID{123456789012345678, 20}
			}
			value, err := scope.New(1, kind, ids)
			require.NoError(t, err)
			a, err := domain.NewAssignment(domain.SubjectTypeUser, 1, 2, domain.WithGrantedBy("admin"), domain.WithScope(value))
			require.NoError(t, err)
			po := NewMapper().ToPO(&a)
			restored, err := NewMapper().ToBO(po)
			require.NoError(t, err)
			got, ok := restored.Scope()
			require.True(t, ok)
			require.Equal(t, value.OrgID(), got.OrgID())
			require.Equal(t, value.Kind(), got.Kind())
			require.Equal(t, value.StoreIDs(), got.StoreIDs())
		})
	}
}

func TestLegacyAssignmentRestoresWithoutImplicitScope(t *testing.T) {
	a, err := NewMapper().ToBO(&AssignmentPO{SubjectType: "user", SubjectID: "1", RoleID: 2, GrantedBy: "admin"})
	require.NoError(t, err)
	_, ok := a.Scope()
	require.False(t, ok)
	po := NewMapper().ToPO(a)
	require.Zero(t, po.OrgID)
	require.Empty(t, po.ScopeKind)
	require.Nil(t, po.ScopeStoreIDs)
}

func TestScopeMappingRejectsMalformedStoredFacts(t *testing.T) {
	for _, raw := range []string{`null`, `{}`, `[1]`, `["0"]`, `["-1"]`, `["9223372036854775808"]`, `[]`, `["2"] trailing`} {
		po := &AssignmentPO{SubjectType: "user", SubjectID: "1", RoleID: 2, GrantedBy: "admin", OrgID: 1, ScopeKind: "stores", ScopeStoreIDs: &raw}
		_, err := NewMapper().ToBO(po)
		require.Error(t, err, raw)
	}
	for _, po := range []*AssignmentPO{
		{OrgID: 1}, {ScopeKind: "all_stores"}, {OrgID: 1, ScopeKind: "unknown"}, {OrgID: ^uint64(0), ScopeKind: "all_stores"},
	} {
		po.SubjectType = "user"
		po.SubjectID = "1"
		po.RoleID = 2
		po.GrantedBy = "admin"
		_, err := NewMapper().ToBO(po)
		require.Error(t, err)
	}
}

func TestRepositoryPreservesCompanyScopedAssignments(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&AssignmentPO{}))
	repo := NewRepository(db)
	ctx := context.Background()
	for _, org := range []meta.ID{1, 2} {
		value, err := scope.New(org, scope.Stores, []meta.ID{10})
		require.NoError(t, err)
		a, err := domain.NewAssignment(domain.SubjectTypeUser, 1, 2, domain.WithGrantedBy("admin"), domain.WithScope(value))
		require.NoError(t, err)
		require.NoError(t, repo.Create(ctx, &a))
		restored, err := repo.FindByID(ctx, a.ID)
		require.NoError(t, err)
		got, ok := restored.Scope()
		require.True(t, ok)
		require.Equal(t, org, got.OrgID())
		require.True(t, got.ContainsStore(org, 10))
		require.Error(t, repo.Create(ctx, &a), "duplicate in same company must fail")
	}
	rows, err := repo.ListBySubject(ctx, domain.SubjectTypeUser, 1)
	require.NoError(t, err)
	require.Len(t, rows, 2)
}
