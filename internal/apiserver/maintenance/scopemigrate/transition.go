package scopemigrate

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"
	assignmentpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/assignment"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
)

// ValidateTransition proves only reviewed assignments changed. The first target
// retains the historical Assignment ID; extra company targets receive new IDs.
// Archived before values are the rollback baseline, not reconstructed defaults.
func ValidateTransition(before, after Snapshot, p Plan, actor meta.ID) error {
	invalid := func(message string) error { return fmt.Errorf("invalid scope transition: %s", message) }
	if actor <= 0 || p.Validate() != nil || p.Fingerprint != before.Hash() || before.IAM.PolicyVersion == math.MaxInt64 || after.IAM.PolicyVersion != before.IAM.PolicyVersion+1 {
		return invalid("actor, plan or policy version")
	}
	oldRest, newRest := before, after
	oldRest.IAM.Assignments = nil
	newRest.IAM.Assignments = nil
	oldRest.IAM.PolicyVersion = 0
	newRest.IAM.PolicyVersion = 0
	if oldRest.Hash() != newRest.Hash() {
		return invalid("non-assignment facts changed")
	}
	oldRows, newRows := map[string]assignmentpo.AssignmentPO{}, map[string]assignmentpo.AssignmentPO{}
	for _, a := range before.IAM.Assignments {
		if a.ID <= 0 {
			return invalid("invalid old assignment ID")
		}
		if _, ok := oldRows[a.ID.String()]; ok {
			return invalid("duplicate old ID")
		}
		oldRows[a.ID.String()] = a
	}
	for _, a := range after.IAM.Assignments {
		if a.ID <= 0 {
			return invalid("invalid new assignment ID")
		}
		if _, ok := newRows[a.ID.String()]; ok {
			return invalid("duplicate new ID")
		}
		newRows[a.ID.String()] = a
	}
	changes := map[string]Change{}
	extra := map[string]Target{}
	for _, c := range p.Changes {
		if len(c.Targets) == 0 {
			return invalid("missing target")
		}
		if _, exists := changes[c.AssignmentID]; exists {
			return invalid("duplicate change")
		}
		changes[c.AssignmentID] = c
		for _, t := range c.Targets[1:] {
			key := c.SubjectID + "/" + c.RoleID + "/" + t.OrgID
			if _, ok := extra[key]; ok {
				return invalid("duplicate new target")
			}
			extra[key] = t
		}
	}
	for id, old := range oldRows {
		next, ok := newRows[id]
		if !ok {
			return invalid("historical assignment removed")
		}
		delete(newRows, id)
		c, changed := changes[id]
		if !changed {
			if !reflect.DeepEqual(old, next) {
				return invalid("unreviewed assignment modified")
			}
			continue
		}
		delete(changes, id)
		if old.DeletedAt != nil || old.OrgID != 0 || old.ScopeKind != "" || old.ScopeStoreIDs != nil || old.SubjectType != "user" || old.SubjectID != c.SubjectID || fmt.Sprint(old.RoleID) != c.RoleID {
			return invalid("reviewed identity changed")
		}
		if !matches(next, c.Targets[0]) || old.Version == math.MaxUint32 || next.Version != old.Version+1 || next.UpdatedBy != actor || next.UpdatedAt.IsZero() || next.UpdatedAt.Before(old.UpdatedAt.Truncate(time.Second)) {
			return invalid("invalid scoped update or audit")
		}
		// Scope and update audit are the only permitted changes on the original ID.
		next.OrgID = old.OrgID
		next.ScopeKind = old.ScopeKind
		next.ScopeStoreIDs = old.ScopeStoreIDs
		next.Version = old.Version
		next.UpdatedAt = old.UpdatedAt
		next.UpdatedBy = old.UpdatedBy
		if !reflect.DeepEqual(old, next) {
			return invalid("historical identity or grant provenance changed")
		}
	}
	if len(changes) != 0 {
		return invalid("missing reviewed assignment")
	}
	for _, a := range newRows {
		key := a.SubjectID + "/" + fmt.Sprint(a.RoleID) + "/" + fmt.Sprint(a.OrgID)
		target, ok := extra[key]
		if !ok || a.SubjectType != "user" || a.DeletedAt != nil || a.DeletedBy != 0 || !matches(a, target) || a.Version != 1 || a.CreatedBy != actor || a.UpdatedBy != actor || a.GrantedBy != "user:"+actor.String() || a.CreatedAt.IsZero() || a.UpdatedAt.IsZero() || a.GrantedAt.IsZero() {
			return invalid("unreviewed new assignment or invalid audit")
		}
		delete(extra, key)
	}
	if len(extra) != 0 {
		return invalid("missing new company assignment")
	}
	return nil
}
func matches(a assignmentpo.AssignmentPO, t Target) bool {
	if fmt.Sprint(a.OrgID) != t.OrgID || scope.Kind(a.ScopeKind) != t.Kind || a.ScopeStoreIDs == nil {
		return false
	}
	var ids []string
	if json.Unmarshal([]byte(*a.ScopeStoreIDs), &ids) != nil || ids == nil {
		return false
	}
	return reflect.DeepEqual(ids, t.StoreIDs)
}
