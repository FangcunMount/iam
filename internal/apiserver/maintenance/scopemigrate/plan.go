// Package scopemigrate bootstraps explicit assignment scopes. It never infers
// company-wide access from historical roles, doctor identity or missing data.
package scopemigrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"
	grantpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/permissiongrant"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/rolemodel"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
)

type Target struct {
	OrgID    string     `json:"org_id"`
	Kind     scope.Kind `json:"kind"`
	StoreIDs []string   `json:"store_ids"`
}
type Mapping struct {
	AssignmentID string   `json:"assignment_id"`
	SubjectID    string   `json:"subject_id"`
	RoleID       string   `json:"role_id"`
	Targets      []Target `json:"targets"`
}
type Input struct {
	Version  int       `json:"version"`
	Mappings []Mapping `json:"mappings"`
}

// Company facts must be read from QS, not from the user-authored mapping file.
type OperatorFact struct {
	ID, OrgID, UserID string
	Active            bool
}
type StoreFact struct {
	ID, OrgID string
	Active    bool
}
type BusinessFacts struct {
	Operators []OperatorFact
	Stores    []StoreFact
}

// PermissionEvidence reports unchanged action grants; it does not claim that
// legacy QS relationship-filtered record visibility was company-wide.
type PermissionEvidence struct {
	GrantID  string `json:"grant_id"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
}
type Change struct {
	Permissions []PermissionEvidence `json:"permissions"`
	BeforeScope string               `json:"before_scope"`

	AssignmentID string   `json:"assignment_id"`
	SubjectID    string   `json:"subject_id"`
	RoleID       string   `json:"role_id"`
	RoleName     string   `json:"role_name"`
	Targets      []Target `json:"targets"`
}
type Plan struct {
	EffectivePermissions []EffectivePermission `json:"effective_permissions"`
	Fingerprint          string                `json:"fingerprint"`
	Changes              []Change              `json:"changes"`
	Issues               []string              `json:"issues"`
}

func (p Plan) Validate() error {
	if len(p.Issues) > 0 {
		return fmt.Errorf("scope migration blocked: %v", p.Issues)
	}
	return nil
}
func Decode(r io.Reader) (Input, error) {
	var input Input
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	if err := d.Decode(&input); err != nil {
		return input, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return input, fmt.Errorf("trailing mapping data")
	}
	if input.Version != 1 {
		return input, fmt.Errorf("unsupported mapping version")
	}
	return input, nil
}
func positive(value string) (meta.ID, error) {
	id, err := meta.ParseID(value)
	if err != nil || id <= 0 || id.String() != value {
		return 0, fmt.Errorf("invalid canonical ID %q", value)
	}
	return id, nil
}
func normalize(t Target) (Target, error) {
	org, err := positive(t.OrgID)
	if err != nil {
		return Target{}, err
	}
	ids := make([]meta.ID, 0, len(t.StoreIDs))
	for _, raw := range t.StoreIDs {
		id, e := positive(raw)
		if e != nil {
			return Target{}, e
		}
		ids = append(ids, id)
	}
	value, err := scope.New(org, t.Kind, ids)
	if err != nil {
		return Target{}, err
	}
	result := Target{OrgID: org.String(), Kind: value.Kind(), StoreIDs: []string{}}
	for _, id := range value.StoreIDs() {
		result.StoreIDs = append(result.StoreIDs, id.String())
	}
	return result, nil
}
func governed(name string) bool { return name == "platform_admin" || strings.HasPrefix(name, "qs:") }
func known(name string) bool {
	switch name {
	case "platform_admin", "qs:admin", "qs:content_manager", "qs:assessment_operator", "qs:evaluation_plan_manager", "qs:result_reviewer":
		return true
	}
	return false
}

// Build requires coverage of every active legacy backend assignment. A single
// old assignment may be explicitly split across multiple company memberships.
// Already scoped assignments are never overwritten by this bootstrap.
func Build(state rolemodel.State, business BusinessFacts, input Input) Plan {
	p := Plan{Changes: []Change{}, Issues: []string{}}
	issue := func(f string, args ...any) { p.Issues = append(p.Issues, fmt.Sprintf(f, args...)) }
	if input.Version != 1 {
		issue("unsupported mapping version")
	}
	if state.PolicyVersion <= 0 {
		issue("invalid policy version")
	}
	business.Operators = append([]OperatorFact(nil), business.Operators...)
	business.Stores = append([]StoreFact(nil), business.Stores...)
	sort.Slice(business.Operators, func(i, j int) bool { return business.Operators[i].ID < business.Operators[j].ID })
	sort.Slice(business.Stores, func(i, j int) bool { return business.Stores[i].ID < business.Stores[j].ID })
	p.Fingerprint = (Snapshot{IAM: state, QS: business}).Hash()
	memberships := map[string]int{}
	stores := map[string]StoreFact{}
	operators := map[string]bool{}
	for _, o := range business.Operators {
		_, idErr := positive(o.ID)
		_, orgErr := positive(o.OrgID)
		_, userErr := positive(o.UserID)
		if idErr != nil || orgErr != nil || userErr != nil || operators[o.ID] {
			issue("invalid or duplicate operator %s", o.ID)
			continue
		}
		operators[o.ID] = true
		if o.Active {
			memberships[o.OrgID+"/"+o.UserID]++
		}
	}
	for _, s := range business.Stores {
		_, idErr := positive(s.ID)
		_, orgErr := positive(s.OrgID)
		_, exists := stores[s.ID]
		if idErr != nil || orgErr != nil || exists {
			issue("invalid or duplicate store %s", s.ID)
			continue
		}
		stores[s.ID] = s
	}
	mappings := map[string]Mapping{}
	for _, m := range input.Mappings {
		if _, err := positive(m.AssignmentID); err != nil {
			issue("invalid assignment mapping ID")
			continue
		}
		if _, ok := mappings[m.AssignmentID]; ok {
			issue("duplicate mapping %s", m.AssignmentID)
			continue
		}
		mappings[m.AssignmentID] = m
	}
	roles := map[uint64]string{}
	for _, r := range state.Roles {
		if r.DeletedAt == nil {
			roles[r.ID.Uint64()] = r.Name
		}
	}
	existing := map[string]bool{}
	for _, a := range state.Assignments {
		if a.DeletedAt == nil && a.OrgID > 0 {
			existing[a.SubjectType+"/"+a.SubjectID+"/"+fmt.Sprint(a.RoleID)+"/"+fmt.Sprint(a.OrgID)] = true
		}
	}
	for _, a := range state.Assignments {
		if a.DeletedAt != nil {
			continue
		}
		name, ok := roles[a.RoleID]
		if !ok {
			issue("assignment %s has no active role", a.ID.String())
			continue
		}
		if !governed(name) {
			continue
		}
		if !known(name) {
			issue("unexpected backend role %s", name)
			continue
		}
		if a.OrgID != 0 || a.ScopeKind != "" || a.ScopeStoreIDs != nil {
			var ids []string
			invalidJSON := false
			if a.ScopeStoreIDs != nil {
				invalidJSON = json.Unmarshal([]byte(*a.ScopeStoreIDs), &ids) != nil || ids == nil
			}
			_, err := normalize(Target{OrgID: fmt.Sprint(a.OrgID), Kind: scope.Kind(a.ScopeKind), StoreIDs: ids})
			if invalidJSON || err != nil {
				issue("invalid existing scope %s", a.ID.String())
			}

			continue
		}
		m, ok := mappings[a.ID.String()]
		if !ok {
			issue("missing mapping for %s (%s:%s %s)", a.ID.String(), a.SubjectType, a.SubjectID, name)
			continue
		}
		delete(mappings, a.ID.String())
		if a.SubjectType != "user" || m.SubjectID != a.SubjectID || m.RoleID != fmt.Sprint(a.RoleID) {
			issue("mapping identity mismatch %s", a.ID.String())
			continue
		}
		if len(m.Targets) == 0 {
			issue("explicit targets required for %s", a.ID.String())
			continue
		}
		c := Change{AssignmentID: a.ID.String(), SubjectID: a.SubjectID, RoleID: m.RoleID, RoleName: name, Targets: []Target{}}
		seen := map[string]bool{}
		for _, target := range m.Targets {
			t, err := normalize(target)
			if err != nil {
				issue("assignment %s: %v", a.ID.String(), err)
				continue
			}
			if seen[t.OrgID] {
				issue("duplicate company target %s/%s", a.ID.String(), t.OrgID)
				continue
			}
			seen[t.OrgID] = true
			if memberships[t.OrgID+"/"+a.SubjectID] != 1 {
				issue("assignment %s lacks unique active Operator in company %s", a.ID.String(), t.OrgID)
			}
			if existing[a.SubjectType+"/"+a.SubjectID+"/"+m.RoleID+"/"+t.OrgID] {
				issue("existing scoped assignment conflicts with %s/%s", a.ID.String(), t.OrgID)
			}
			for _, id := range t.StoreIDs {
				s, ok := stores[id]
				if !ok || !s.Active || s.OrgID != t.OrgID {
					issue("assignment %s has unavailable company store %s", a.ID.String(), id)
				}
			}
			c.Targets = append(c.Targets, t)
		}
		c.BeforeScope = "unconfigured"
		c.Permissions = []PermissionEvidence{}
		for _, g := range state.Grants {
			if g.RoleID != a.RoleID || g.DeletedAt != nil || g.RevokedAt != nil {
				continue
			}
			grant, err := (grantpo.Mapper{}).ToBO(&g)
			if err != nil {
				issue("invalid active Grant %s: %v", g.ID.String(), err)
				continue
			}
			c.Permissions = append(c.Permissions, PermissionEvidence{GrantID: g.ID.String(), Resource: grant.ResourceKeyString(), Action: grant.ActionString()})
		}
		sort.Slice(c.Permissions, func(i, j int) bool { return c.Permissions[i].GrantID < c.Permissions[j].GrantID })
		sort.Slice(c.Targets, func(i, j int) bool { return c.Targets[i].OrgID < c.Targets[j].OrgID })
		p.Changes = append(p.Changes, c)
	}
	for id := range mappings {
		issue("mapping %s does not reference a legacy backend assignment", id)
	}
	sort.Slice(p.Changes, func(i, j int) bool { return p.Changes[i].AssignmentID < p.Changes[j].AssignmentID })
	if len(p.Issues) == 0 {
		var err error
		p.EffectivePermissions, err = effectivePermissions(state, p.Changes)
		if err != nil {
			issue("effective permission report: %v", err)
		}
	}
	sort.Strings(p.Issues)
	return p
}

// EqualPlan compares canonical reviewed plans without ignoring issues.
func EqualPlan(a, b Plan) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
