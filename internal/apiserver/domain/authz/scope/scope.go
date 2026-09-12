// Package scope expresses the data range attached to a role assignment.
// QS owns store existence and object ownership; IAM only carries granted IDs.
package scope

import (
	"slices"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v5/internal/pkg/code"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
)

type Kind string

const (
	AllStores Kind = "all_stores"
	Stores    Kind = "stores"
)

// Scope belongs to one company. Its zero value grants no data access.
// Fields are private so callers cannot bypass construction or mutate a shared snapshot.
type Scope struct {
	orgID    meta.ID
	kind     Kind
	storeIDs []meta.ID
}

func New(orgID meta.ID, kind Kind, storeIDs []meta.ID) (Scope, error) {
	invalid := func(message string) (Scope, error) {
		return Scope{}, perrors.WithCode(code.ErrInvalidArgument, "%s", message)
	}
	if orgID <= 0 {
		return invalid("scope company is required")
	}
	switch kind {
	case AllStores:
		if len(storeIDs) != 0 {
			return invalid("all-stores scope cannot contain selected stores")
		}
	case Stores:
		if len(storeIDs) == 0 {
			return invalid("selected-stores scope requires stores")
		}
	default:
		return invalid("unsupported scope kind")
	}
	ids := slices.Clone(storeIDs)
	for _, id := range ids {
		if id <= 0 {
			return invalid("scope store ID must be positive")
		}
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if len(ids) == 0 {
		ids = nil
	}
	return Scope{orgID: orgID, kind: kind, storeIDs: ids}, nil
}

func (s Scope) OrgID() meta.ID      { return s.orgID }
func (s Scope) Kind() Kind          { return s.kind }
func (s Scope) StoreIDs() []meta.ID { return slices.Clone(s.storeIDs) }
func (s Scope) IsZero() bool        { return s.orgID <= 0 || s.kind == "" }

// ContainsStore checks a known current store. An unassigned object is not a
// store and must never acquire access through an implicit wildcard.
func (s Scope) ContainsStore(orgID, storeID meta.ID) bool {
	if s.IsZero() || s.orgID != orgID || storeID <= 0 {
		return false
	}
	if s.kind == AllStores {
		return true
	}
	_, found := slices.BinarySearch(s.storeIDs, storeID)
	return s.kind == Stores && found
}

// Union combines ranges only after the caller has matched the resource and
// action of each source assignment. It never combines different companies.
func (s Scope) Union(other Scope) (Scope, error) {
	if s.IsZero() || other.IsZero() || s.orgID != other.orgID {
		return Scope{}, perrors.WithCode(code.ErrInvalidArgument, "scope union requires the same company and valid ranges")
	}
	if s.kind == AllStores || other.kind == AllStores {
		return New(s.orgID, AllStores, nil)
	}
	return New(s.orgID, Stores, append(s.StoreIDs(), other.storeIDs...))
}
