package scope

import (
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"reflect"
	"testing"
)

func TestScopeRejectsAmbiguousOrInvalidRanges(t *testing.T) {
	for _, tc := range []struct {
		org    meta.ID
		kind   Kind
		stores []meta.ID
	}{
		{0, AllStores, nil}, {-1, AllStores, nil}, {1, "", nil}, {1, "global", nil},
		{1, Stores, nil}, {1, AllStores, []meta.ID{2}}, {1, Stores, []meta.ID{0}}, {1, Stores, []meta.ID{-2}},
	} {
		if _, err := New(tc.org, tc.kind, tc.stores); err == nil {
			t.Errorf("accepted invalid scope: %+v", tc)
		}
	}
}

func TestScopeCanonicalizesWithoutExposingMutableState(t *testing.T) {
	input := []meta.ID{3, 2, 3}
	s, err := New(1, Stores, input)
	if err != nil {
		t.Fatal(err)
	}
	input[0] = 99
	ids := s.StoreIDs()
	ids[0] = 100
	if !reflect.DeepEqual(s.StoreIDs(), []meta.ID{2, 3}) {
		t.Fatal("scope mutated or not normalized")
	}
	if !s.ContainsStore(1, 2) || s.ContainsStore(2, 2) || s.ContainsStore(1, 99) {
		t.Fatal("company/store boundary violated")
	}
}

func TestAllStoresStillRequiresCompanyAndAssignedStore(t *testing.T) {
	s, err := New(1, AllStores, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !s.ContainsStore(1, 999) || s.ContainsStore(2, 999) || s.ContainsStore(1, 0) || s.ContainsStore(1, -1) {
		t.Fatal("all stores widened company or unassigned access")
	}
	if (Scope{}).ContainsStore(1, 1) {
		t.Fatal("zero scope grants access")
	}
}

func TestUnionPreservesCompanyAndInputs(t *testing.T) {
	a, _ := New(1, Stores, []meta.ID{2})
	b, _ := New(1, Stores, []meta.ID{3})
	union, err := a.Union(b)
	if err != nil {
		t.Fatal(err)
	}
	if !union.ContainsStore(1, 2) || !union.ContainsStore(1, 3) || a.ContainsStore(1, 3) {
		t.Fatal("invalid union or input mutated")
	}
	all, _ := New(1, AllStores, nil)
	union, err = union.Union(all)
	if err != nil || !union.ContainsStore(1, 999) {
		t.Fatal("company-wide union failed")
	}
	other, _ := New(2, Stores, []meta.ID{2})
	if _, err = a.Union(other); err == nil {
		t.Fatal("cross-company union accepted")
	}
	if _, err = a.Union(Scope{}); err == nil {
		t.Fatal("missing range accepted")
	}
}
