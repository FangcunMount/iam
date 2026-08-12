package suggest

import "testing"

func TestScopePolicy_AllProfile(t *testing.T) {
	p := ScopePolicy{}
	term := NewProfileSearchTerm(1, "a", nil, 1, 999, []int64{999})
	if !p.Allows(ProfileAccessScope{AllProfile: true}, term) {
		t.Fatal("expected allow AllProfile")
	}
}

func TestScopePolicy_ProfileIDs(t *testing.T) {
	p := ScopePolicy{}
	term := NewProfileSearchTerm(5, "a", nil, 1, 0, nil)
	if !p.Allows(ProfileAccessScope{ProfileIDs: []int64{5}}, term) {
		t.Fatal("expected allow by ProfileID")
	}
	if p.Allows(ProfileAccessScope{ProfileIDs: []int64{9}}, term) {
		t.Fatal("expected deny when ProfileID not in list")
	}
}

func TestScopePolicy_OrgOperator(t *testing.T) {
	p := ScopePolicy{}
	term := NewProfileSearchTerm(1, "a", nil, 1, 20, []int64{100, 200})
	if !p.Allows(ProfileAccessScope{OrgIDs: []int64{20}}, term) {
		t.Fatal("expected org match")
	}
	if !p.Allows(ProfileAccessScope{OperatorID: 200}, term) {
		t.Fatal("expected owner operator match")
	}
	if p.Allows(ProfileAccessScope{OperatorID: 999}, term) {
		t.Fatal("expected deny wrong operator")
	}
}

func TestScopePolicy_EmptyScopeDenies(t *testing.T) {
	p := ScopePolicy{}
	term := NewProfileSearchTerm(1, "a", nil, 1, 1, []int64{1})
	if p.Allows(ProfileAccessScope{}, term) {
		t.Fatal("empty scope should deny")
	}
}

func TestScopePolicy_OrgIDZeroIgnored(t *testing.T) {
	p := ScopePolicy{}
	term := NewProfileSearchTerm(1, "a", nil, 1, 0, nil)
	if p.Allows(ProfileAccessScope{OrgIDs: []int64{1}}, term) {
		t.Fatal("term org_id 0 must not match org list")
	}
}

func TestScopePolicy_CompiledLargeProfileIDs(t *testing.T) {
	p := ScopePolicy{}
	ids := make([]int64, 500)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	scope := CompileProfileAccessScope(ProfileAccessScope{ProfileIDs: ids})
	term := NewProfileSearchTerm(250, "a", nil, 1, 1, nil)
	if !p.AllowsCompiled(scope, term) {
		t.Fatal("expected hit in compiled profile set")
	}
	term2 := NewProfileSearchTerm(99999, "b", nil, 1, 1, nil)
	if p.AllowsCompiled(scope, term2) {
		t.Fatal("expected miss")
	}
}
