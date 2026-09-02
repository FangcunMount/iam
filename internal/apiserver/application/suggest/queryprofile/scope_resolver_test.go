package queryprofile_test

import (
	"context"
	"errors"
	"testing"

	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
)

type stubFacts struct {
	facts visibility.AuthorizationFacts
	err   error
	calls int
}

func (s *stubFacts) ReadAuthorizationFacts(context.Context, visibility.Principal) (visibility.AuthorizationFacts, error) {
	s.calls++
	if s.err != nil {
		return visibility.AuthorizationFacts{}, s.err
	}
	return s.facts, nil
}

type stubVisibility struct {
	ids   []int64
	err   error
	calls int
}

func (v *stubVisibility) VisibleProfileIDs(context.Context, visibility.Principal) ([]int64, error) {
	v.calls++
	if v.err != nil {
		return nil, v.err
	}
	return append([]int64(nil), v.ids...), nil
}

func TestScopeResolverNilFactsReturnsEmptyScope(t *testing.T) {
	r := appquery.NewScopeResolver(nil, nil)
	scope, err := r.ResolveScope(context.Background(), visibility.Principal{OperatorID: 100, OrgID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if scope.IsAllProfiles() || scope.AllowsMobileSearch() {
		t.Fatalf("scope = %#v", scope)
	}
}

func TestScopeResolverPlatformListSkipsVisibility(t *testing.T) {
	vis := &stubVisibility{ids: []int64{99}}
	r := appquery.NewScopeResolver(&stubFacts{
		facts: visibility.AuthorizationFacts{PlatformListAllowed: true, PlatformMobileSearchAllowed: true},
	}, vis)
	scope, err := r.ResolveScope(context.Background(), visibility.Principal{OperatorID: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !scope.IsAllProfiles() || !scope.AllowsMobileSearch() {
		t.Fatalf("scope = %#v", scope)
	}
	if vis.calls != 0 {
		t.Fatalf("visibility calls = %d, want 0", vis.calls)
	}
}

func TestScopeResolverTenantMobileFromFacts(t *testing.T) {
	r := appquery.NewScopeResolver(&stubFacts{
		facts: visibility.AuthorizationFacts{TenantMobileSearchAllowed: true},
	}, nil)
	scope, err := r.ResolveScope(context.Background(), visibility.Principal{
		OperatorID: 100,
		OrgID:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if scope.IsAllProfiles() {
		t.Fatal("AllProfiles should be false for tenant user")
	}
	if !scope.AllowsMobileSearch() {
		t.Fatal("mobile search should be allowed")
	}
	if scope.OperatorID() != 100 {
		t.Fatalf("OperatorID = %d", scope.OperatorID())
	}
}

func TestScopeResolverVisibilityMerge(t *testing.T) {
	r := appquery.NewScopeResolver(&stubFacts{}, &stubVisibility{ids: []int64{7, 9, 7}})
	scope, err := r.ResolveScope(context.Background(), visibility.Principal{OperatorID: 100})
	if err != nil {
		t.Fatal(err)
	}
	pids := scope.ProfileIDs()
	if len(pids) != 2 {
		t.Fatalf("ProfileIDs = %v", pids)
	}
}

func TestScopeResolverFactsErrorPropagates(t *testing.T) {
	want := errors.New("authz down")
	r := appquery.NewScopeResolver(&stubFacts{err: want}, nil)
	_, err := r.ResolveScope(context.Background(), visibility.Principal{OperatorID: 100})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v", err)
	}
}

func TestScopeResolverVisibilityErrorPropagates(t *testing.T) {
	want := errors.New("visibility failed")
	r := appquery.NewScopeResolver(&stubFacts{}, &stubVisibility{err: want})
	_, err := r.ResolveScope(context.Background(), visibility.Principal{OperatorID: 100})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v", err)
	}
}
