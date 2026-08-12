package policylint

import (
	"context"
	"testing"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/scope"
	"github.com/stretchr/testify/require"
)

func TestLinterAcceptsValidExactPermissionFact(t *testing.T) {
	t.Parallel()

	res := mustResource(t, "iam:identity:collection:users", []string{"read", "list"}, []scope.Kind{scope.KindAll})
	linter := NewLinter(
		factReaderStub{facts: []PermissionFact{{
			RoleName:    "iam:admin",
			TenantID:    "tenant-a",
			ResourceKey: res.KeyString(),
			Action:      "read|list",
			Scope:       "all:*",
		}}},
		resourceCatalogStub{resources: []*resource.Resource{res}},
	)

	report, err := linter.Lint(context.Background())

	require.NoError(t, err)
	require.Empty(t, report.Findings)
}

func TestLinterReportsUnsupportedActionAndScopeKind(t *testing.T) {
	t.Parallel()

	res := mustResource(t, "iam:identity:collection:users", []string{"read"}, []scope.Kind{scope.KindAll})
	linter := NewLinter(
		factReaderStub{facts: []PermissionFact{{
			RoleName:    "iam:admin",
			TenantID:    "tenant-a",
			ResourceKey: res.KeyString(),
			Action:      "delete",
			Scope:       "origin:demo",
		}}},
		resourceCatalogStub{resources: []*resource.Resource{res}},
	)

	report, err := linter.Lint(context.Background())

	require.NoError(t, err)
	requireFindingCode(t, report, FindingUnsupportedAction)
	requireFindingCode(t, report, FindingUnsupportedScopeKind)
}

func TestLinterReportsMissingResource(t *testing.T) {
	t.Parallel()

	linter := NewLinter(
		factReaderStub{facts: []PermissionFact{{
			RoleName:    "iam:admin",
			TenantID:    "tenant-a",
			ResourceKey: "iam:identity:collection:users",
			Action:      "read",
			Scope:       "all:*",
		}}},
		resourceCatalogStub{},
	)

	report, err := linter.Lint(context.Background())

	require.NoError(t, err)
	requireFindingCode(t, report, FindingMissingResource)
}

func TestLinterReportsUncheckableActionPattern(t *testing.T) {
	t.Parallel()

	res := mustResource(t, "iam:identity:collection:users", []string{"read"}, []scope.Kind{scope.KindAll})
	linter := NewLinter(
		factReaderStub{facts: []PermissionFact{{
			RoleName:    "iam:admin",
			TenantID:    "tenant-a",
			ResourceKey: res.KeyString(),
			Action:      "[",
			Scope:       "all:*",
		}}},
		resourceCatalogStub{resources: []*resource.Resource{res}},
	)

	report, err := linter.Lint(context.Background())

	require.NoError(t, err)
	requireFindingCode(t, report, FindingUncheckableActionRegex)
}

func TestLinterScansWildcardResourcePatterns(t *testing.T) {
	t.Parallel()

	res := mustResource(t, "qs:questionnaire:collection:questionnaires", []string{"read"}, []scope.Kind{scope.KindAll})
	linter := NewLinter(
		factReaderStub{facts: []PermissionFact{{
			RoleName:    "qs:admin",
			TenantID:    "tenant-a",
			ResourceKey: "qs:*:*:*",
			Action:      "read",
			Scope:       "all:*",
		}}},
		resourceCatalogStub{resources: []*resource.Resource{res}},
	)

	report, err := linter.Lint(context.Background())

	require.NoError(t, err)
	require.Empty(t, report.Findings)
}

type factReaderStub struct {
	facts []PermissionFact
}

func (s factReaderStub) ListPermissionFacts(context.Context) ([]PermissionFact, error) {
	return append([]PermissionFact(nil), s.facts...), nil
}

type resourceCatalogStub struct {
	resources []*resource.Resource
}

func (s resourceCatalogStub) FindByKey(_ context.Context, key string) (*resource.Resource, error) {
	for _, res := range s.resources {
		if res.KeyString() == key {
			return res, nil
		}
	}
	return nil, nil
}

func (s resourceCatalogStub) List(context.Context, resource.ResourceFilter) ([]*resource.Resource, int64, error) {
	return append([]*resource.Resource(nil), s.resources...), int64(len(s.resources)), nil
}

func mustResource(t *testing.T, key string, actions []string, scopeKinds []scope.Kind) *resource.Resource {
	t.Helper()

	res, err := resource.NewResource(key, actions, resource.WithScopeKinds(scopeKinds))
	require.NoError(t, err)
	return &res
}

func requireFindingCode(t *testing.T, report Report, code FindingCode) {
	t.Helper()

	for _, finding := range report.Findings {
		if finding.Code == code {
			return
		}
	}
	require.Failf(t, "missing finding code", "findings = %#v, want code %s", report.Findings, code)
}
